package main

import (
	"crypto/subtle"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
)

// RFC 6598: Shared Address Space (CGNAT), not captured by net.IP.IsPrivate
var cgnat = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

type Handler struct {
	configuration Configuration
	locks         map[string]*sync.Mutex
}

func NewHandler(configPath string) (*Handler, error) {
	configuration, err := NewConfiguration(configPath)
	if err != nil {
		return nil, err
	}

	this := new(Handler)
	this.configuration = *configuration

	//prevent a theoretical race condition
	this.locks = make(map[string]*sync.Mutex, len(configuration.hosts))
	for i := range configuration.hosts {
		this.locks[configuration.hosts[i].FQDN()] = new(sync.Mutex)
	}
	return this, nil
}

func validateQuery(query url.Values) (map[string]string, error) {
	params := []string{"fqdn", "ipv4", "token"}
	validQuery := make(map[string]string)

	for _, param := range params {
		queryItems, ok := query[param]

		if !ok || len(queryItems) < 1 {
			return nil, fmt.Errorf("parameter %q is missing", param)
		}

		validQuery[param] = queryItems[0]
	}

	return validQuery, nil
}

// DynDNSHandler provides a handler function for the web package
func (h *Handler) DynDNSHandler(w http.ResponseWriter, r *http.Request) {
	query, err := validateQuery(r.URL.Query())

	if err != nil {
		fmt.Println(err)
		http.Error(w, "Missing or invalid parameter(s)", http.StatusBadRequest)
		return
	}

	//validate ip by parsing it in go
	ip := net.ParseIP(query["ipv4"])

	if ip == nil {
		fmt.Printf("Invalid IP Address %q\n", query["ipv4"])
		http.Error(w, "Missing or invalid parameter(s)", http.StatusBadRequest)
		return
	}

	//validate ipv4 by converting
	ip = ip.To4()

	if ip == nil {
		fmt.Printf("IP Address %q is not v4\n", query["ipv4"])
		http.Error(w, "Missing or invalid parameter(s)", http.StatusBadRequest)
		return
	}

	if !ip.IsGlobalUnicast() || ip.IsPrivate() || cgnat.Contains(ip) {
		fmt.Printf("IP Address %q is not a public unicast address\n", query["ipv4"])
		http.Error(w, "Missing or invalid parameter(s)", http.StatusBadRequest)
		return
	}

	//validate fqdn by searching it in the config file
	host, err := h.configuration.GetHost(query["fqdn"])

	if err != nil {
		fmt.Println(err)
		http.Error(w, "Not authorized to set this fqdn", http.StatusForbidden)
		return
	}

	//validate token in constant time
	if subtle.ConstantTimeCompare([]byte(host.WebToken), []byte(query["token"])) != 1 {
		fmt.Printf("Invalid token supplied for fqdn %q\n", host.FQDN())
		http.Error(w, "Not authorized to set this fqdn", http.StatusForbidden)
		return
	}

	// lock FQDN max until timeout
	lock := h.locks[host.FQDN()]
	lock.Lock()
	defer lock.Unlock()

	if host.IP == ip.String() {
		fmt.Println("IP did not change")
		fmt.Fprint(w, "success")
		return
	}

	fmt.Printf("Changing IP Address from %v to %v\n", host.IP, ip.String())

	err = host.updateIP(ip)
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Something unexpected happened", http.StatusInternalServerError)
		return
	}

	fmt.Println("Success")
	fmt.Fprint(w, "success")
}

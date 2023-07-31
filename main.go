package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Wifx/gonetworkmanager/v2"
)

type interfaceMap struct {
	wireless  string
	wired     string
	tailscale string
}

type duckDns struct {
	apiToken string
	client   *http.Client
}

func newDuckDns() (*duckDns, error) {
	return &duckDns{
		apiToken: os.Getenv("DUCKDNS_API_TOKEN"),
		client:   http.DefaultClient,
	}, nil
}

func (d *duckDns) updateDNSEntry(ctx context.Context, ip, domain string) error {
	url := fmt.Sprintf("https://duckdns.org/update/%s/%s", domain, d.apiToken)

	if ip != "" {
		url = url + "/" + ip
	}

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	res, err := d.client.Do(req)
	if err != nil {
		log.Printf("failed to update %s:%s; err: %s", domain, ip, err)
		return err
	}

	defer res.Body.Close()

	responseBody, _ := io.ReadAll(res.Body)
	if string(responseBody) != "OK" {
		log.Printf("error response from duckdns API server")
		return fmt.Errorf("invalid response from duckdns: %s", string(responseBody))
	}

	return nil
}

func privateIps() (interfaceMap, error) {
	imap := interfaceMap{}
	nm, err := gonetworkmanager.NewNetworkManager()
	if err != nil {
		return imap, err
	}

	devices, err := nm.GetDevices()
	if err != nil {
		return imap, err
	}

	for _, device := range devices {
		deviceType, _ := device.GetPropertyDeviceType()
		interfaceName, _ := device.GetPropertyIpInterface()
		ipv4Config, _ := device.GetPropertyIP4Config()
		deviceAddrs, _ := ipv4Config.GetPropertyAddresses()

		if deviceType == gonetworkmanager.NmDeviceTypeEthernet {
			imap.wired = deviceAddrs[0].Address
		}

		if deviceType == gonetworkmanager.NmDeviceTypeWifi {
			imap.wireless = deviceAddrs[0].Address

		}

		if deviceType == gonetworkmanager.NmDeviceTypeTun && strings.HasPrefix(interfaceName, "tailscale") {
			imap.tailscale = deviceAddrs[0].Address
		}
	}

	return imap, nil
}

func main() {
	d, _ := newDuckDns()
	ticker := time.NewTicker(time.Duration(5 * time.Minute))
	var imap interfaceMap
	var err error
	last := interfaceMap{"", "", ""}

	for {
		if imap, err = privateIps(); err != nil {
			continue
		}

		if imap.wired == last.wired && imap.wireless == last.wireless && imap.tailscale == last.tailscale {
			log.Printf("IP addresses did not change, skipping execution")
			continue
		}

		// update the last IPs
		last.wired = imap.wired
		last.wireless = imap.wireless
		last.tailscale = imap.tailscale

		addr := imap.wired
		if addr == "" {
			addr = imap.wireless
		}

		// update public IP
		log.Printf("Executing public DNS update")
		d.updateDNSEntry(context.TODO(), "", "peeche.duckdns.org")

		// update the local IP
		if addr != "" {
			log.Printf("Executing private DNS update with %s", addr)
			d.updateDNSEntry(context.TODO(), addr, "dekh.duckdns.org")
		}

		// update tailscale IP if the interface is up
		if imap.tailscale != "" {
			log.Printf("Executing tailscale DNS update with %s", imap.tailscale)
			d.updateDNSEntry(context.TODO(), imap.tailscale, "maut.duckdns.org")
		}

		<-ticker.C
	}
}

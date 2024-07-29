package main

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// TODO: Check for duplicate shortname
// TODO: Move to config file
// TODO: Generate /etc/ssl/private/list.txt
// TODO: Health check URI setting
// TODO: Service container port setting
// TODO: Default redirect editable setting

// Settings
const ADMIN_USER = "changeme"
const ADMIN_PASSWORD = "something-random-and-secure"
const POSTGRESQL_USER = "localuser"
const POSTGRESQL_PASS = "localroot"
const POSTGRESQL_DB = "localdb"

// Build Information
const BuildVersion string = "0.0.1"

var BuildDate string
var BuildGoVersion string
var BuildGitHash string

type DevOpsService struct {
	ID        int               `json:"id"`
	Name      string            `json:"name"`
	Shortname string            `json:"shortname"`
	Domain    string            `json:"domain"`
	Env       map[string]string `json:"environment"`
}

var Services []DevOpsService = []DevOpsService{
	{
		Name:      "exampleapp",
		Shortname: "APP",
		Domain:    "app.example.com",
	},
	{
		Name:      "exampleapi",
		Shortname: "API",
		Domain:    "api.example.com",
		Env: map[string]string{
			"NODE_ENV":       "production",
			"LISTEN_ADDRESS": "0.0.0.0",
			"LISTEN_PORT":    "8080",
			"COOKIE_SECRET":  "cookie-secret",
			"CSRF_SECRET":    "csrf-secret",
			"SQL_URI":        fmt.Sprintf("postgresql://%s:%s@postgresql:5432/%s?connect_timeout=10&sslmode=disable", POSTGRESQL_USER, POSTGRESQL_PASS, POSTGRESQL_DB),
			"DEBUG":          "",
		},
	},
}

func fileUpdate(services []DevOpsService) {
	lines := []string{
		"#!/usr/bin/env bash",
		"",
		"mkdir -p ./tarballs/;",
	}
	for _, service := range services {
		lines = append(lines, snipUpdateService(service)...)
	}
	writeStrings("update.sh", lines)
}

func snipUpdateService(service DevOpsService) []string {
	return []string{
		"",
		fmt.Sprintf("if [ -f %s.tar ]; then", service.Name),
		fmt.Sprintf("	sudo podman-compose down %s;", service.Name),
		fmt.Sprintf("	sudo podman load --input %s.tar;", service.Name),
		fmt.Sprintf("	mv %s.tar ./tarballs/;", service.Name),
		fmt.Sprintf("	sudo podman-compose up %s -d;", service.Name),
		"fi",
	}
}

func fileDockerCompose(services []DevOpsService) {
	lines := []string{
		"---",
		"# https://docs.docker.com/compose/compose-file/",
		"",
		"services:",
		"  postgresql:",
		"    image: postgres:16.3-alpine3.20",
		"    container_name: postgresql",
		"    restart: always",
		"    # ports:",
		"    #   # Host:Container",
		`    #   - "5432:5432"`,
		"    volumes:",
		"      - ./data_postgresql:/var/lib/postgresql/data",
		"      #- ./postgresql_scripts:/docker-entrypoint-initdb.d",
		"    environment:",
		fmt.Sprintf("      POSTGRES_USER: %s", POSTGRESQL_USER),
		fmt.Sprintf("      POSTGRES_PASSWORD: %s", POSTGRESQL_PASS),
		fmt.Sprintf("      POSTGRES_DB: %s", POSTGRESQL_DB),
		"    networks:",
		"      appnet:",
		"        ipv4_address: 192.168.100.10",
	}
	for _, service := range services {
		lines = append(lines, snipDockerComposeService(service)...)
	}
	linesEnd := []string{
		"",
		"networks:",
		"  appnet:",
		"    ipam:",
		"      #driver: bridge",
		"      config:",
		`        - subnet: "192.168.100.0/24"`,
	}
	lines = append(lines, linesEnd...)
	writeStrings("docker-compose.yml", lines)
}

func snipDockerComposeService(service DevOpsService) []string {
	top := []string{
		"",
		fmt.Sprintf("  %s:", service.Name),
		fmt.Sprintf("    image: localhost/%s:latest", service.Name),
		fmt.Sprintf("    container_name: %s", service.Name),
		"    restart: always",
		"    ports:",
		fmt.Sprintf(`      - "%d:8080"`, (8080 + service.ID)),
	}
	envLines := []string{}
	if len(service.Env) > 0 {
		envLines = append(envLines, "    environment:")
		for envName, envValue := range service.Env {
			envLines = append(envLines, fmt.Sprintf(`      %s: "%s"`, envName, envValue))
		}
	}
	bottom := []string{
		"    networks:",
		"      appnet:",
		fmt.Sprintf("        ipv4_address: 192.168.100.%d", (80 + service.ID)),
	}
	one := append(top, envLines...)
	return append(one, bottom...)
}

func fileHaproxyCfg(services []DevOpsService) {
	lines := []string{
		"",
	}
	lines = append(lines, snipHaproxyGlobal()...)
	lines = append(lines, snipHaproxyDefaults()...)
	lines = append(lines, snipHaproxyStats()...)
	lines = append(lines, snipHaproxyFrontendHttp(services)...)
	lines = append(lines, snipHaproxyFrontendHttps(services)...)
	lines = append(lines, snipHaproxyBackends(services)...)
	writeStrings("haproxy.cfg", lines)
}

func snipHaproxyGlobal() []string {
	return []string{
		"global",
		"	# refuse to start if insufficient FDs/memory",
		"	strict-limits",
		"	# ulimit for nofile is 1048576 (1mm+)",
		"	# sysctl -a fs.nr_open = 1048576",
		"	# systemd limits may be handled differently",
		"	#",
		"	# By the way, there are at least 2 TCP connections",
		"	# for each frontend connection. 1 client, 1 server backend so...",
		"	# fd-hard-limit needs to be 2x maxconn value + some for overhead",
		"	maxconn 50000",
		"	fd-hard-limit 1024000",
		"	log     /dev/log syslog notice",
		"	chroot  /var/lib/haproxy",
		"	user    haproxy",
		"	group   haproxy",
		"	#daemon",
		"",
		"	stats  socket /run/haproxy-stats.socket mode 660 level admin expose-fd listeners",
		"	stats  timeout 2s",
		"",
		"	# Usually Set Automatically",
		"	#nbthread  2",
		"	hard-stop-after    1m",
		"	log-send-hostname  HAProxy",
		"	#server-state-base  /etc/haproxy",
		"	#server-state-file  state.cfg",
		"	#server-state-file /etc/haproxy/state.cfg",
		"	# Backup Command - Preserves Server State on Reload",
		"	# socat /run/haproxy-stats.socket - <<< 'show servers state' > /etc/haproxy/state.cfg",
		"",
		"	# Default SSL material locations",
		"	ca-base  /etc/ssl/certs",
		"	crt-base /etc/ssl/private",
		"",
		"	# See: https://ssl-config.mozilla.org/#server=haproxy&version=3.0.3&config=intermediate&openssl=3.0.13&guideline=5.7",
		"	ssl-default-bind-ciphers         ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384:ECDHE-ECDSA-CHACHA20-POLY1305:ECDHE-RSA-CHACHA20-POLY1305:DHE-RSA-AES128-GCM-SHA256:DHE-RSA-AES256-GCM-SHA384",
		"	ssl-default-bind-ciphersuites    TLS_AES_128_GCM_SHA256:TLS_AES_256_GCM_SHA384:TLS_CHACHA20_POLY1305_SHA256",
		"	ssl-default-bind-options         ssl-min-ver TLSv1.2 no-tls-tickets",
		"	ssl-default-server-ciphers       ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384:ECDHE-ECDSA-CHACHA20-POLY1305:ECDHE-RSA-CHACHA20-POLY1305:DHE-RSA-AES128-GCM-SHA256:DHE-RSA-AES256-GCM-SHA384",
		"	ssl-default-server-ciphersuites  TLS_AES_128_GCM_SHA256:TLS_AES_256_GCM_SHA384:TLS_CHACHA20_POLY1305_SHA256",
		"	ssl-default-server-options       ssl-min-ver TLSv1.2 no-tls-tickets",
		"",
	}
}

func snipHaproxyDefaults() []string {
	return []string{
		"defaults",
		"	log        global",
		"	mode       http",
		"	option     httplog",
		"	#mode      tcp",
		"	#option    tcplog",
		"	option     dontlognull",
		"	timeout    connect      5s",
		"	timeout    client       10s",
		"	timeout    server       10s",
		"	timeout    http-request 10s",
		"	errorfile  400          /etc/haproxy/errors/400.http",
		"	errorfile  403          /etc/haproxy/errors/403.http",
		"	errorfile  408          /etc/haproxy/errors/408.http",
		"	errorfile  500          /etc/haproxy/errors/500.http",
		"	errorfile  502          /etc/haproxy/errors/502.http",
		"	errorfile  503          /etc/haproxy/errors/503.http",
		"	errorfile  504          /etc/haproxy/errors/504.http",
		"",
	}
}

func snipHaproxyStats() []string {
	return []string{
		"frontend stats",
		"	bind     :8443  name HAProxy-8443  alpn h2,http/1.1  ssl crt-list /etc/ssl/private/list.txt",
		"	mode     http",
		"	log      global",
		"	#max-session-srv-conns 10",
		"	#option logasap",
		"	#no option http-keep-alive",
		"	#option httpclose",
		"",
		"	stats  enable",
		"	#stats  maxconn  20",
		"	stats  hide-version",
		"	stats  refresh 5s",
		"	stats  show-node",
		"	stats  uri /stats",
		fmt.Sprintf("	stats  auth %s:%s", ADMIN_USER, ADMIN_PASSWORD),
		"	stats  admin if TRUE",
		"",
	}
}

func snipHaproxyFrontendHttp(services []DevOpsService) []string {
	lines := []string{
		"frontend http",
		"	bind     :80  name HAProxy-80  alpn h2,http/1.1",
		"	timeout  client 10s",
		"	option   dontlog-normal",
		"	#option  httpclose",
		"	#option  http-server-close",
	}
	for _, service := range services {
		// Root domain
		if strings.Count(service.Domain, ".") == 1 {
			lines = append(lines, []string{
				fmt.Sprintf("	acl  %s_ROOT    var(txn.txnhost) -m str -i %s", service.Shortname, service.Domain),
				fmt.Sprintf("	acl  %s_WWW     var(txn.txnhost) -m str -i www.%s", service.Shortname, service.Domain),
			}...)
			// Subdomain
		} else {
			// Check for www
			first3 := service.Domain[0:3]
			if first3 == "www" {
				// Add www and also root
				lines = append(lines, []string{
					fmt.Sprintf("	acl  %s_ROOT    var(txn.txnhost) -m str -i %s", service.Shortname, service.Domain[4:]),
					fmt.Sprintf("	acl  %s_WWW     var(txn.txnhost) -m str -i %s", service.Shortname, service.Domain),
				}...)
			} else {
				// Just add the listed domain
				lines = append(lines, []string{
					fmt.Sprintf("	acl  %s_MAIN    var(txn.txnhost) -m str -i %s", service.Shortname, service.Domain),
				}...)
			}
		}
	}

	lines = append(lines, []string{
		"	acl  ACL_Default  var(txn.txnhost) -m reg -i .*",
		"	http-request      set-var(txn.txnhost) hdr(host)",
	}...)

	for _, service := range services {
		// Root domain
		if strings.Count(service.Domain, ".") == 1 {
			lines = append(lines, []string{
				fmt.Sprintf("	http-request   redirect scheme https code 301 if %s_ROOT", service.Shortname),
				fmt.Sprintf("	http-request   redirect location https://%s/ code 301 if %s_WWW", service.Domain, service.Shortname),
			}...)
			// Subdomain
		} else {
			// Check for www
			first3 := service.Domain[0:3]
			if first3 == "www" {
				// Add www and also root
				lines = append(lines, []string{
					fmt.Sprintf("	http-request   redirect location https://%s/ code 301 if %s_ROOT", service.Domain, service.Shortname),
					fmt.Sprintf("	http-request   redirect scheme https code 301 if %s_WWW", service.Shortname),
				}...)
			} else {
				// Just add the listed domain
				lines = append(lines, []string{
					fmt.Sprintf("	http-request   redirect scheme https code 301 if %s_MAIN", service.Shortname),
				}...)
			}
		}
	}

	lines = append(lines, []string{
		"	http-request   redirect location https://www.google.com/ code 301 if ACL_Default",
		"",
	}...)
	return lines
}

func snipHaproxyFrontendHttps(services []DevOpsService) []string {
	lines := []string{
		"frontend https",
		"	bind     :443  name HAProxy-443  alpn h2,http/1.1 ssl crt-list /etc/ssl/private/list.txt",
		"	#bind    :443  name HAProxy-443  alpn h2,http/1.1 ssl crt STAR_example_com.combo",
		"	timeout  client 10s",
		"	option dontlog-normal",
		"	option forwardfor",
		"",
		"	acl https ssl_fc",
		"	# http-request redirect scheme https unless { ssl_fc }",
		"",
		"	acl valid_method method GET POST PUT DELETE OPTIONS HEAD",
		"	http-request deny if !valid_method",
		"",
		"	http-request			del-header  X-Forwarded-Proto",
		"	http-request			set-header  X-Forwarded-Proto http if !https",
		"	http-request			set-header  X-Forwarded-Proto https if https",
		"	http-request			del-header  X-Forwarded-For",
		"	http-after-response		del-header  Server",
		"	http-after-response		del-header  ETag",
		"	http-after-response		del-header  X-Powered-By",
		`	http-after-response		set-header  Strict-Transport-Security "max-age=15552000; includeSubDomains"`,
		"	#http-request			del-header  ^Host*",
		"	#http-request			set-header  Host www.example.com",
		"",
		"	http-request set-var(txn.txnhost) hdr(host)",
	}

	lines1 := []string{}
	lines2 := []string{}
	lines3 := []string{}

	for _, service := range services {
		// Root domain
		if strings.Count(service.Domain, ".") == 1 {
			lines1 = append(lines1, []string{
				fmt.Sprintf("	acl  %s_ROOT  var(txn.txnhost) -m str -i %s", service.Shortname, service.Domain),
				fmt.Sprintf("	acl  %s_WWW   var(txn.txnhost) -m str -i www.%s", service.Shortname, service.Domain),
			}...)
			lines2 = append(lines2, []string{
				fmt.Sprintf("	http-request	redirect location https://%s/ code 301 if %s_WWW", service.Domain, service.Shortname),
			}...)
			lines3 = append(lines3, []string{
				fmt.Sprintf("	use_backend    %s_L7      if %s_ROOT", service.Domain, service.Shortname),
			}...)
			// Subdomain
		} else {
			// Check for www
			first3 := service.Domain[0:3]
			if first3 == "www" {
				// Add www and also root
				lines1 = append(lines1, []string{
					fmt.Sprintf("	acl  %s_ROOT  var(txn.txnhost) -m str -i %s", service.Shortname, service.Domain[4:]),
					fmt.Sprintf("	acl  %s_WWW   var(txn.txnhost) -m str -i %s", service.Shortname, service.Domain),
				}...)
				lines2 = append(lines2, []string{
					fmt.Sprintf("	http-request	redirect location https://%s/ code 301 if %s_ROOT", service.Domain, service.Shortname),
				}...)
				lines3 = append(lines3, []string{
					fmt.Sprintf("	use_backend    %s_L7      if %s_WWW", service.Domain, service.Shortname),
				}...)
			} else {
				// Just add the listed domain
				lines1 = append(lines1, []string{
					fmt.Sprintf("	acl  %s_MAIN	var(txn.txnhost) -m str -i %s", service.Shortname, service.Domain),
				}...)
				lines3 = append(lines3, []string{
					fmt.Sprintf("	use_backend    %s_L7      if %s_MAIN", service.Domain, service.Shortname),
				}...)
			}
		}
	}

	lines = append(lines, lines1...)
	lines = append(lines, lines2...)
	lines = append(lines, lines3...)
	return append(lines, []string{
		"	default_backend default_L7",
		"",
	}...)
}

func snipHaproxyBackends(services []DevOpsService) []string {
	lines := []string{
		"",
		"#####################",
		"### BACKENDS HERE ###",
		"#####################",
	}
	for _, service := range services {
		lines = append(lines, snipHaproxyBackend(service)...)
	}
	lines = append(lines, []string{
		"",
		"backend default_L7",
		"	mode          http",
		"	http-request  redirect location https://www.google.com/ code 301",
	}...)
	return lines
}

func snipHaproxyBackend(service DevOpsService) []string {
	return []string{
		"",
		fmt.Sprintf("backend %s_L7", service.Domain),
		"	mode             http",
		"	log              global",
		"	balance          roundrobin",
		"	timeout connect  5s",
		"	timeout server   5s",
		"	retries          2",
		"	#load-server-state-from-file  global",
		"",
		"	option      httpchk",
		"	option      log-health-checks",
		fmt.Sprintf("	http-check  connect port %d addr 127.0.0.1", (8080 + service.ID)),
		fmt.Sprintf("	http-check  send meth GET uri /health ver HTTP/1.1 hdr Host %s", service.Domain),
		"	http-check  expect status 200",
		fmt.Sprintf("	server      backend_%s 127.0.0.1:%d id %d check inter 1000", service.Name, (8080 + service.ID), (100 + service.ID)),
	}
}

func writeStrings(filename string, lines []string) {
	//strings.Join(fileSlice[:], "\n");
	f, err := os.Create(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	for _, line := range lines {
		_, err := f.WriteString(line + "\n")
		if err != nil {
			log.Fatal(err)
		}
	}
}

func main() {
	idx := 0
	for _ = range Services {
		idx++
		Services[idx-1].ID = idx
	}
	fileUpdate(Services)
	fileDockerCompose(Services)
	fileHaproxyCfg(Services)
}

package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	qrcode "github.com/skip2/go-qrcode"
)

func main() {
	cfg, err := loadConfig(ConfigFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if err := os.MkdirAll(cfg.UploadDir, 0755); err != nil {
		log.Fatalf("Failed to create upload dir: %v", err)
	}

	// Sidecar is created but NOT auto-started — user starts it from the dashboard
	sidecar := NewSidecar(cfg)

	// Wire up HTTP handlers
	handlers := NewHandlers(cfg, sidecar)

	mux := http.NewServeMux()
	// AI endpoints
	mux.HandleFunc("/ask", handlers.HandleAsk)
	// API endpoints for admin dashboard
	mux.HandleFunc("/api/status", handlers.HandleStatus)
	mux.HandleFunc("/api/sysinfo", handlers.HandleSysInfo)
	mux.HandleFunc("/api/llama/stop", handlers.HandleLlamaStop)
	mux.HandleFunc("/api/llama/start", handlers.HandleLlamaStart)
	mux.HandleFunc("/api/llama/releases", handlers.HandleLlamaReleases)
	mux.HandleFunc("/api/llama/select", handlers.HandleLlamaSelect)
	mux.HandleFunc("/api/download", handlers.HandleDownload)
	mux.HandleFunc("/api/model/select", handlers.HandleModelSelect)
	mux.HandleFunc("/api/vision/toggle", handlers.HandleVisionToggle)
	mux.HandleFunc("/api/config", handlers.HandleConfig)
	mux.HandleFunc("/api/cert/regenerate", handlers.HandleCertRegenerate)
	mux.HandleFunc("/api/llama/test", handlers.HandleLlamaTest)
	mux.HandleFunc("/api/qr", handlers.HandleQR)
	mux.HandleFunc("/api/qr-url", handlers.HandleQRURL)
	// Static files (index.html → UA redirect, eye.html, admin.html)
	mux.Handle("/", http.FileServer(http.Dir("static")))

	// Detect mobile-accessible IP for QR code (WSL-aware)
	lanIP := getMobileIP()
	mobileURL := fmt.Sprintf("https://%s:%s", lanIP, cfg.HTTPSPort)

	printQR(mobileURL)
	log.Printf("Dashboard : http://localhost:%s", cfg.HTTPPort)
	log.Printf("Mobile URL: %s  (scan the QR code above)", mobileURL)

	// Graceful shutdown on SIGINT / SIGTERM
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Println("Shutting down...")
		sidecar.Stop()
		os.Exit(0)
	}()

	// Start HTTP (for local access without cert warnings)
	go func() {
		log.Fatal(http.ListenAndServe(":"+cfg.HTTPPort, mux))
	}()

	// Start HTTPS (required for camera access on mobile browsers)
	log.Fatal(http.ListenAndServeTLS(
		":"+cfg.HTTPSPort,
		".ssl/cert.pem",
		".ssl/key.pem",
		mux,
	))
}

// getLANIP returns the first non-loopback IPv4 address of the machine.
func getLANIP() string {
	ips := getLANIPs()
	if len(ips) > 0 {
		return ips[0].String()
	}
	return "localhost"
}

// getLANIPs returns all non-loopback IPv4 addresses of the machine.
func getLANIPs() []net.IP {
	var result []net.IP
	ifaces, err := net.Interfaces()
	if err != nil {
		return result
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			if ip4 := ip.To4(); ip4 != nil {
				result = append(result, ip4)
			}
		}
	}
	return result
}

// getMobileIP returns the IP a smartphone should connect to.
// In WSL2 the Linux internal IP is not reachable from the LAN;
// the Windows host IP (read from /etc/resolv.conf) must be used instead.
func getMobileIP() string {
	if isWSL() {
		if ip := wslHostIP(); ip != "" {
			return ip
		}
	}
	return getLANIP()
}

// isWSL reports whether the process is running inside Windows Subsystem for Linux.
func isWSL() bool {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	lower := strings.ToLower(string(data))
	return strings.Contains(lower, "microsoft") || strings.Contains(lower, "wsl")
}

// wslHostIP returns the Windows host's real LAN IPv4 address from WSL2.
// Tries powershell.exe first, falls back to ipconfig.exe parsing.
func wslHostIP() string {
	// --- Primary: powershell.exe (precise, filter by prefix length) ---
	out, err := exec.Command("powershell.exe", "-NoProfile", "-Command",
		"(Get-NetIPAddress -AddressFamily IPv4 | "+
			"Where-Object { $_.IPAddress -notlike '127.*' -and "+
			"$_.IPAddress -notlike '169.*' -and "+
			"$_.IPAddress -notlike '172.*' } | "+
			"Sort-Object -Property PrefixLength | "+
			"Select-Object -ExpandProperty IPAddress -First 1)").Output()
	if err == nil {
		if ip := strings.TrimSpace(string(out)); net.ParseIP(ip) != nil {
			return ip
		}
	}

	// --- Fallback: ipconfig.exe (works on any Windows locale) ---
	// Lines look like:
	//   IPv4 Address. . . : 192.168.0.65          (EN)
	//   Indirizzo IPv4. . : 192.168.0.65           (IT)
	out, err = exec.Command("ipconfig.exe").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, "IPv4") {
			continue
		}
		idx := strings.LastIndex(line, ":")
		if idx < 0 {
			continue
		}
		ip := strings.TrimSpace(line[idx+1:])
		if net.ParseIP(ip) == nil {
			continue
		}
		// Skip loopback, link-local, and WSL2 virtual adapter range
		if strings.HasPrefix(ip, "127.") ||
			strings.HasPrefix(ip, "169.") ||
			strings.HasPrefix(ip, "172.") {
			continue
		}
		return ip
	}
	return ""
}

// printQR prints the QR code for the mobile URL to stdout.
func printQR(url string) {
	q, err := qrcode.New(url, qrcode.Medium)
	if err != nil {
		log.Println("Could not generate QR code:", err)
		return
	}
	fmt.Println("\n--- Scan this QR code with your phone ---")
	fmt.Println(q.ToSmallString(false))
	fmt.Printf("URL: %s\n\n", url)
}

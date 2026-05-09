package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
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

	// Start llama-server sidecar
	sidecar := NewSidecar(cfg)
	if err := sidecar.Start(); err != nil {
		log.Printf("WARNING: llama-server could not start: %v", err)
		log.Println("Server will run but /ask endpoint will return 503 until the model is ready.")
	}

	// Wire up HTTP handlers
	handlers := NewHandlers(cfg, sidecar)

	mux := http.NewServeMux()
	// AI endpoints
	mux.HandleFunc("/ask", handlers.HandleAsk)
	// API endpoints for admin dashboard
	mux.HandleFunc("/api/status", handlers.HandleStatus)
	mux.HandleFunc("/api/sysinfo", handlers.HandleSysInfo)
	mux.HandleFunc("/api/llama/stop", handlers.HandleLlamaStop)
	mux.HandleFunc("/api/download", handlers.HandleDownload)
	mux.HandleFunc("/api/config", handlers.HandleConfig)
	mux.HandleFunc("/api/cert/regenerate", handlers.HandleCertRegenerate)
	// Static files (index.html → UA redirect, eye.html, admin.html)
	mux.Handle("/", http.FileServer(http.Dir("static")))

	// Detect LAN IP for QR code
	lanIP := getLANIP()
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

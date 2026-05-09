# GemmaLink — WSL2 Port Forwarding Setup
# Run as Administrator in PowerShell on Windows host.
# Execute once per WSL2 session (WSL2 IP changes on reboot).
#
#   PowerShell -ExecutionPolicy Bypass -File wsl-portforward.ps1

$ports = @(8080, 8443)

# Get current WSL2 internal IP
$wslIP = (wsl hostname -I 2>$null).Trim().Split(' ')[0]
if (-not $wslIP) {
    Write-Error "Could not get WSL2 IP. Is WSL2 running?"
    exit 1
}
Write-Host "WSL2 IP: $wslIP" -ForegroundColor Cyan

foreach ($port in $ports) {
    # Remove existing rule (ignore errors)
    netsh interface portproxy delete v4tov4 listenport=$port listenaddress=0.0.0.0 2>$null | Out-Null

    # Add forward: Windows 0.0.0.0:<port> → WSL2:<port>
    netsh interface portproxy add v4tov4 `
        listenaddress=0.0.0.0 listenport=$port `
        connectaddress=$wslIP connectport=$port

    # Firewall rule (idempotent — delete+add)
    netsh advfirewall firewall delete rule name="GemmaLink $port" 2>$null | Out-Null
    netsh advfirewall firewall add rule `
        name="GemmaLink $port" protocol=TCP dir=in action=allow localport=$port

    Write-Host "Port $port forwarded and firewall opened." -ForegroundColor Green
}

Write-Host ""
Write-Host "Done. Smartphone can now reach https://192.168.0.65:8443/eye.html" -ForegroundColor Yellow

# Assicura che lo script giri come Amministratore
if (-not ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Warning "Esegui PowerShell come Amministratore per pulire il firewall!"
    Exit
}

# Script di Reset Universale per la porta e per il nome dell'app
Write-Host "Inizio pulizia regole firewall per GemmaLink..." -ForegroundColor Cyan

Remove-NetFirewallRule -DisplayName "*GemmaLink*" -ErrorAction SilentlyContinue
Remove-NetFirewallRule -DisplayName "*gemmalink*" -ErrorAction SilentlyContinue
Remove-NetFirewallRule -DisplayName "*llama*" -ErrorAction SilentlyContinue

# Cancella per nome file (copre qualsiasi cartella di estrazione dello ZIP)
Get-NetFirewallRule | Where-Object { $_.ApplicationPath -like "*gemmalink.exe" -or $_.ApplicationPath -like "*llama-server.exe" } | Remove-NetFirewallRule -ErrorAction SilentlyContinue

# Cancella per porta (sicurezza extra per evitare conflitti sulla 9381)
Get-NetFirewallPortFilter | Where-Object { $_.LocalPort -eq "9381" } | Get-NetFirewallRule | Remove-NetFirewallRule -ErrorAction SilentlyContinue

Write-Host "Pulizia completata! Il firewall è pronto per il nuovo IP." -ForegroundColor Green

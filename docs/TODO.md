# reset fw rules
- creare un file go per il reset delle regole del firewal:
```go
package main

import (
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
)

func ResetFirewallFromScript() error {
	// Costruisce il percorso relativo allo script partendo dalla cartella dell'app
	scriptRelativePath := filepath.Join("script", "win-reset-fw-rules.ps1")
	
	// Converte il percorso in assoluto (richiesto da PowerShell quando si eleva con RunAs)
	scriptAbsolutePath, err := filepath.Abs(scriptRelativePath)
	if err != nil {
		return fmt.Errorf("impossibile determinare il percorso assoluto dello script: %w", err)
	}

	// Configura i parametri per Start-Process:
	// -FilePath: indica di lanciare powershell
	// -ArgumentList: passa i parametri a powershell, incluso il flag -File per eseguire lo script ed evitare il blocco della ExecutionPolicy
	// -Verb RunAs: richiede i privilegi di amministratore (pop-up UAC di Windows)
	args := []string{
		"Start-Process", "powershell",
		"-ArgumentList", fmt.Sprintf(`"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", "%s"`, scriptAbsolutePath),
		"-Verb", "RunAs",
	}

	cmd := exec.Command("powershell", "-Command", args...)
	
	err = cmd.Run()
	if err != nil {
		log.Printf("Errore durante l'esecuzione dello script del firewall elevato: %v", err)
		return err
	}
	
	return nil
}
```
Attenzione> lo script deve funzionare anche su linux
- aggiungere un pulsante nella scheda config per l'esecuzione del comando vicino alla creazione del certificato
- valutare se tenere in config.json l'ultimo ip dell'host (caso dhcp) per invitare l'utente a ricreare certificato e resettare il firewall
- Spostare la sezione dalla config alla dashboard principale
#!/bin/bash

# GemmaLink Firewall Tool - Data Driven Version
CONFIG_FILE="config.json"
APP_NAME="GemmaLink"

if [[ $EUID -ne 0 ]]; then
   echo "Errore: Eseguire come root (sudo $0 {set|reset})"
   exit 1
fi

if [[ ! -f "$CONFIG_FILE" ]]; then
    echo "Errore: $CONFIG_FILE non trovato nella directory corrente."
    exit 1
fi

# Estrazione porte usando sed (evita la dipendenza da jq)
HTTP_PORT=$(grep '"http_port":' "$CONFIG_FILE" | sed -E 's/.*"([^"]+)".*/\1/')
HTTPS_PORT=$(grep '"https_port":' "$CONFIG_FILE" | sed -E 's/.*"([^"]+)".*/\1/')

ACTION=$1

apply_firewall() {
    local port=$1
    local proto=$2
    
    if command -v ufw >/dev/null; then
        ufw allow "$port/tcp" comment "$APP_NAME $proto"
    elif command -v firewall-cmd >/dev/null; then
        firewall-cmd --permanent --add-port="$port/tcp"
    fi
}

remove_firewall() {
    local port=$1
    
    if command -v ufw >/dev/null; then
        ufw delete allow "$port/tcp"
    elif command -v firewall-cmd >/dev/null; then
        firewall-cmd --permanent --remove-port="$port/tcp"
    fi
}

case "$ACTION" in
    set)
        echo "Configurazione porte da $CONFIG_FILE: HTTP=$HTTP_PORT, HTTPS=$HTTPS_PORT"
        [[ -n "$HTTP_PORT" ]] && apply_firewall "$HTTP_PORT" "HTTP"
        [[ -n "$HTTPS_PORT" ]] && apply_firewall "$HTTPS_PORT" "HTTPS"
        
        # Ricarica firewalld se presente
        command -v firewall-cmd >/dev/null && firewall-cmd --reload
        echo "Regole applicate con successo."
        ;;
    reset)
        echo "Rimozione regole per $APP_NAME..."
        [[ -n "$HTTP_PORT" ]] && remove_firewall "$HTTP_PORT"
        [[ -n "$HTTPS_PORT" ]] && remove_firewall "$HTTPS_PORT"
        
        command -v firewall-cmd >/dev/null && firewall-cmd --reload
        echo "Regole rimosse."
        ;;
    *)
        echo "Utilizzo: sudo $0 {set|reset}"
        ;;
esac
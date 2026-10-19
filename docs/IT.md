# Installazione rapida

Supporto iniziale: **reMarkable 2, firmware 3.27.3.0**. Non installare su Paper Pro
o su firmware diversi: l'installer li rifiuta prima di riavviare l'interfaccia.

Su Windows apri PowerShell e incolla:

```powershell
& ([scriptblock]::Create((Invoke-RestMethod 'https://raw.githubusercontent.com/noire342/remarkable-quickshare/v0.1.0/install.ps1')))
```

Inserisci l'IP del tablet, conferma la sua chiave SSH e digita la password root
mostrata in Aiuto → Copyright e licenze. Per USB puoi lasciare `10.11.99.1`.
Non servono Python, Go o installazioni aggiuntive oltre al client OpenSSH di Windows.

L'installer verifica il pacchetto, prepara sul tablet la copia locale di Xochitl,
salva un backup e abilita l'avvio automatico. In caso di errore prova a ripristinare
l'installazione precedente. Non distribuiamo binari del firmware reMarkable.

Per inviare: tablet sveglio, stessa rete Wi-Fi, PDF/EPUB → Condividi → Quick Share
→ **reMarkable 2 XXXX**. Il documento entra in My Files automaticamente.
I file vengono accettati senza conferma sul tablet: usa reti fidate.

Per aggiornare, riesegui l'installer. Per disinstallare:

```powershell
.\install.ps1 -HostAddress IP_DEL_TABLET -Uninstall
```

La disinstallazione conserva documenti, identità, file in attesa e backup.
Gli aggiornamenti del firmware richiedono una nuova verifica di compatibilità.

Se il Galaxy non vede subito il tablet, lascia aperta la schermata d'invio per
20–30 secondi e controlla che Quick Share non disconnetta il Wi-Fi.

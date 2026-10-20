// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Lorenzo Garrone (noire342)
import QtQuick
import QtQuick.Layouts
import ark.controls as ArkControls

Item {
    id: page
    property var controller: null
    ArkControls.PlaceholderBar { id: placeholder; width: parent.width }
    Flickable {
        anchors { top: placeholder.bottom; bottom: parent.bottom; left: parent.left; right: parent.right }
        contentWidth: width
        contentHeight: content.implicitHeight + 120
        clip: true
        flickableDirection: Flickable.VerticalFlick
        Column {
            id: content
            x: 64; y: 56; width: parent.width - 128; spacing: 34
            ArkControls.Title { type: ArkControls.Title.Large; text: "Quick Share"; width: parent.width }
            Text {
                width: parent.width
                text: "Ricevi PDF ed EPUB tramite Quick Share e aggiungili automaticamente alla libreria."
                font.pixelSize: 28; wrapMode: Text.Wrap; color: "black"; textFormat: Text.PlainText
            }
            ModeChoice {
                title: "Chiedi sempre"
                description: "Controlla il codice di verifica e scegli Accetta o Rifiuta per ogni invio."
                selected: !page.controller || page.controller.mode === "ask"
                available: !!page.controller && page.controller.connected && !page.controller.busy
                onChosen: page.controller.setMode("ask")
            }
            ModeChoice {
                title: "Accetta automaticamente"
                description: "Ricevi senza conferma da qualsiasi dispositivo raggiungibile sulla rete. Usa questa modalità su reti fidate."
                selected: !!page.controller && page.controller.mode === "auto"
                available: !!page.controller && page.controller.connected && !page.controller.busy
                onChosen: page.controller.setMode("auto")
            }
            Text {
                width: parent.width
                text: "La scelta viene salvata per i prossimi invii e mantenuta dopo il riavvio. Una richiesta già in attesa continua a richiedere conferma."
                font.pixelSize: 25; wrapMode: Text.Wrap; color: "#555555"; textFormat: Text.PlainText
            }
            Text {
                width: parent.width
                text: page.controller && page.controller.connected ? "Ricevitore collegato. Tieni il tablet sveglio e collegato allo stesso Wi-Fi del mittente." : "Collegamento al ricevitore in corso…"
                font.pixelSize: 25; wrapMode: Text.Wrap; color: "#555555"; textFormat: Text.PlainText
            }
            Text {
                width: parent.width
                visible: !!page.controller && page.controller.errorMessage.length > 0
                text: page.controller ? page.controller.errorMessage : ""
                font.pixelSize: 25; wrapMode: Text.Wrap; color: "black"; textFormat: Text.PlainText
            }
        }
    }
}

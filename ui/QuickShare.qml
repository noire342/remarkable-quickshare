// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Lorenzo Garrone (noire342)
import QtQuick
import QtQuick.Window
import QtQuick.Controls
import QtQuick.Layouts
import "Config.js" as Config

Item {
    id: controller
    objectName: "remarkable-quickshare-controller"
    property bool screenAvailable: false
    property var defaultDialogHost: null
    property var settingsDialogHost: null
    readonly property var dialogHost: settingsDialogHost || defaultDialogHost
    property string mode: "ask"
    property bool connected: false
    property bool busy: false
    property string errorMessage: ""
    property var pending: null
    property double revision: 0
    property var pollingRequest: null
    property int pollGeneration: 0
    property string presentedRequest: ""
    property bool decisionEnabled: false
    property bool initialized: false

    function formatSize(bytes) {
        if (bytes < 1000) return bytes + " B";
        if (bytes < 1000000) return (bytes / 1000).toFixed(1) + " kB";
        return (bytes / 1000000).toFixed(1) + " MB";
    }
    function updateState(state) {
        if (!state || !state.settings || (state.settings.acceptance_mode !== "ask" && state.settings.acceptance_mode !== "auto")) return;
        mode = state.settings.acceptance_mode;
        revision = state.revision || 0;
        connected = true;
        errorMessage = "";
        var nextPending = state.pending || null;
        var requestId = nextPending ? nextPending.id : "";
        if (!pending || pending.id !== requestId) pending = nextPending;
        if (requestId !== presentedRequest) {
            presentedRequest = requestId;
            decisionEnabled = false;
            if (requestId) touchGuard.restart();
        }
    }
    function poll() {
        if (!initialized || pollingRequest) return;
        var generation = ++pollGeneration;
        var request = new XMLHttpRequest();
        pollingRequest = request;
        request.onreadystatechange = function() {
            if (request.readyState !== XMLHttpRequest.DONE || generation !== pollGeneration) return;
            pollingRequest = null;
            if (request.status === 200) {
                try { updateState(JSON.parse(request.responseText)); }
                catch (error) { connected = false; pending = null; }
                retry.interval = connected ? 30 : 2000;
            } else {
                connected = false;
                pending = null;
                errorMessage = "Ricevitore non disponibile. Riprova tra poco.";
                retry.interval = 2000;
            }
            retry.restart();
        };
        request.open("GET", Config.endpoint + "/v1/state?after=" + revision + "&wait=25&ready=" + (screenAvailable ? "1" : "0"));
        request.setRequestHeader("Authorization", "Bearer " + Config.token);
        request.send();
    }
    function refresh() {
        retry.stop();
        if (pollingRequest) {
            ++pollGeneration;
            pollingRequest.abort();
            pollingRequest = null;
        }
        poll();
    }
    function submit(method, path, body) {
        if (busy) return;
        busy = true;
        var request = new XMLHttpRequest();
        request.onreadystatechange = function() {
            if (request.readyState !== XMLHttpRequest.DONE) return;
            busy = false;
            if (request.status === 200) {
                try { updateState(JSON.parse(request.responseText)); }
                catch (error) { errorMessage = "Risposta del ricevitore non valida."; }
            } else if (request.status === 409) {
                pending = null;
                errorMessage = "La richiesta è scaduta o è stata annullata.";
            } else {
                errorMessage = "Impossibile salvare la scelta. Riprova.";
            }
            refresh();
        };
        request.open(method, Config.endpoint + path);
        request.setRequestHeader("Authorization", "Bearer " + Config.token);
        request.setRequestHeader("Content-Type", "application/json");
        request.send(JSON.stringify(body));
    }
    function setMode(value) {
        if (connected && (value === "ask" || value === "auto")) submit("PUT", "/v1/settings", {acceptance_mode: value});
    }
    function decide(value, id) {
        if (busy || !pending || pending.id !== id || !screenAvailable || !decisionEnabled) return;
        decisionEnabled = false;
        submit("POST", "/v1/requests/" + encodeURIComponent(id) + "/decision", {decision: value});
    }
    onScreenAvailableChanged: {
        if (!screenAvailable) { decisionEnabled = false; pending = null; }
        refresh();
    }
    Component.onCompleted: {
        initialized = true;
        console.log("Quick Share confirmation UI loaded");
        poll();
    }
    Component.onDestruction: {
        initialized = false;
        retry.stop();
        ++pollGeneration;
        if (pollingRequest) pollingRequest.abort();
    }
    Timer { id: retry; interval: 2000; onTriggered: controller.poll() }
    Timer {
        id: touchGuard
        interval: 700
        onTriggered: controller.decisionEnabled = !!controller.pending && controller.screenAvailable
    }

    Popup {
        id: approvalWindow
        parent: controller.dialogHost
        modal: true
        dim: true
        closePolicy: Popup.NoAutoClose
        visible: controller.screenAvailable && controller.connected && !!controller.pending
        width: Math.min((parent ? parent.width : 1404) - 96, 1050)
        height: Math.min((parent ? parent.height : 1872) - 96, implicitHeight)
        x: Math.round(((parent ? parent.width : 1404) - width) / 2)
        y: Math.round(((parent ? parent.height : 1872) - height) / 2)
        padding: 48
        enter: null
        exit: null
        Overlay.modal: Rectangle { color: "#88ffffff" }
        background: Rectangle { color: "white"; border.color: "black"; border.width: 3 }
        contentItem: ColumnLayout {
            id: popupContent
            spacing: 28
            Text { text: "Quick Share"; font.pixelSize: 46; font.bold: true; color: "black"; textFormat: Text.PlainText }
            Text {
                Layout.fillWidth: true
                text: controller.pending ? "Invio da " + controller.pending.sender : ""
                font.pixelSize: 30; wrapMode: Text.Wrap; color: "black"; textFormat: Text.PlainText
            }
            Text {
                Layout.fillWidth: true
                text: controller.pending ? controller.pending.files.length + (controller.pending.files.length === 1 ? " documento · " : " documenti · ") + controller.formatSize(controller.pending.total_size) : ""
                font.pixelSize: 26; color: "#444444"; textFormat: Text.PlainText
            }
            Flickable {
                Layout.fillWidth: true
                Layout.preferredHeight: Math.min(fileNames.implicitHeight, 240)
                contentWidth: width
                contentHeight: fileNames.implicitHeight
                clip: true
                flickableDirection: Flickable.VerticalFlick
                Column {
                    id: fileNames
                    width: parent.width
                    spacing: 12
                    Repeater {
                        model: controller.pending ? controller.pending.files : []
                        delegate: Text {
                            required property var modelData
                            width: fileNames.width
                            text: modelData.name; wrapMode: Text.WrapAnywhere
                            font.pixelSize: 25; color: "black"; textFormat: Text.PlainText
                        }
                    }
                }
            }
            Rectangle { Layout.fillWidth: true; height: 1; color: "#aaaaaa" }
            Text { text: "Codice di verifica"; font.pixelSize: 28; color: "black"; textFormat: Text.PlainText }
            Text {
                text: controller.pending ? controller.pending.pin : ""
                Layout.alignment: Qt.AlignHCenter
                font.pixelSize: 96; font.bold: true; font.letterSpacing: 14; color: "black"; textFormat: Text.PlainText
            }
            Text {
                Layout.fillWidth: true
                text: "Controlla che il codice coincida sul telefono. Accetta solo se riconosci l’invio. La richiesta scade dopo un minuto."
                font.pixelSize: 25; wrapMode: Text.Wrap; color: "#444444"; textFormat: Text.PlainText
            }
            RowLayout {
                Layout.fillWidth: true
                spacing: 24
                ActionButton {
                    property string pressedRequest: ""
                    text: "Rifiuta"; Layout.fillWidth: true
                    enabled: controller.decisionEnabled && !controller.busy
                    onPressed: pressedRequest = controller.pending ? controller.pending.id : ""
                    onClicked: controller.decide("reject", pressedRequest)
                }
                ActionButton {
                    property string pressedRequest: ""
                    text: "Accetta"; primary: true; Layout.fillWidth: true
                    enabled: controller.decisionEnabled && !controller.busy
                    onPressed: pressedRequest = controller.pending ? controller.pending.id : ""
                    onClicked: controller.decide("accept", pressedRequest)
                }
            }
        }
    }
}

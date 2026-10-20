// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Lorenzo Garrone (noire342)
import QtQuick
import QtQuick.Controls

Button {
    id: button
    property bool primary: false
    property real textSize: 30
    implicitHeight: 88
    implicitWidth: 260
    padding: 18
    font.pixelSize: textSize
    contentItem: Text {
        text: button.text
        font: button.font
        color: !button.enabled ? "#888888" : button.primary ? "white" : "black"
        horizontalAlignment: Text.AlignHCenter
        verticalAlignment: Text.AlignVCenter
        textFormat: Text.PlainText
    }
    background: Rectangle {
        color: button.primary && button.enabled ? "black" : "white"
        border.color: button.enabled ? "black" : "#aaaaaa"
        border.width: 2
        radius: 4
    }
}

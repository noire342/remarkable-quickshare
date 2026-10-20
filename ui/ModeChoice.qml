// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Lorenzo Garrone (noire342)
import QtQuick
import ark.controls as ArkControls

Item {
    id: choice
    property string title: ""
    property string description: ""
    property bool selected: false
    property bool available: true
    signal chosen()
    width: parent.width
    implicitHeight: labels.implicitHeight
    opacity: available ? 1 : 0.5
    Column {
        id: labels
        width: parent.width
        spacing: 12
        ArkControls.RadioButton {
            width: parent.width
            text: choice.title
            checked: choice.selected
            checkable: false
            autoExclusive: false
            enabled: choice.available
            onClicked: choice.chosen()
        }
        Text {
            x: 64
            width: parent.width - x
            text: choice.description
            font.pixelSize: 25
            wrapMode: Text.Wrap
            textFormat: Text.PlainText
            color: "#444444"
            MouseArea { anchors.fill: parent; enabled: choice.available; onClicked: choice.chosen() }
        }
    }
}

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

Item {
    id: root
    implicitHeight: 66

    property var scenes: []
    property string currentSceneId: ""
    property var backend: null

    signal sceneSelected(string sceneId)

    function accent(item) {
        return item && item.accent ? item.accent : "#76D8FF"
    }

    Rectangle {
        anchors.fill: parent
        radius: 18
        color: Qt.rgba(0.07, 0.11, 0.18, 0.70)
        border.color: Qt.rgba(1, 1, 1, 0.08)
    }

    RowLayout {
        anchors.fill: parent
        anchors.margins: 10
        spacing: 8

        Repeater {
            model: root.scenes

            delegate: Button {
                required property var modelData
                Layout.fillHeight: true
                Layout.preferredWidth: 228
                checkable: true
                checked: modelData.id === root.currentSceneId
                onClicked: root.sceneSelected(modelData.id)

                background: Rectangle {
                    radius: 16
                    color: checked ? Qt.rgba(0.18, 0.26, 0.36, 0.96) : Qt.rgba(0.10, 0.14, 0.21, 0.92)
                    border.width: 1
                    border.color: checked ? Qt.rgba(1, 1, 1, 0.16) : Qt.rgba(1, 1, 1, 0.06)
                }

                contentItem: ColumnLayout {
                    anchors.fill: parent
                    anchors.leftMargin: 14
                    anchors.rightMargin: 14
                    anchors.topMargin: 8
                    anchors.bottomMargin: 8
                    spacing: 1

                    RowLayout {
                        Layout.fillWidth: true
                        spacing: 8
                        Rectangle {
                            width: 8
                            height: 8
                            radius: 4
                            color: root.accent(modelData)
                            Layout.alignment: Qt.AlignVCenter
                        }
                        Text {
                            Layout.fillWidth: true
                            text: modelData.title || ""
                            color: "#F6FAFF"
                            font.pixelSize: 13
                            font.bold: true
                            elide: Text.ElideRight
                        }
                    }

                    Text {
                        Layout.fillWidth: true
                        text: modelData.subtitle || ""
                        color: "#C9D7E8"
                        font.pixelSize: 10
                        elide: Text.ElideRight
                    }

                    RowLayout {
                        Layout.fillWidth: true
                        spacing: 6
                        Text {
                            text: modelData.weatherLabel || modelData.weatherState || ""
                            color: root.accent(modelData)
                            font.pixelSize: 10
                            font.bold: true
                        }
                        Rectangle {
                            implicitWidth: 1
                            implicitHeight: 10
                            color: Qt.rgba(1, 1, 1, 0.12)
                        }
                        Text {
                            Layout.fillWidth: true
                            text: modelData.primaryAction ? modelData.primaryAction.label : ""
                            color: "#9FB6CF"
                            font.pixelSize: 10
                            elide: Text.ElideRight
                        }
                    }
                }
            }
        }

        Item {
            Layout.fillWidth: true
            visible: root.scenes.length === 0

            Text {
                anchors.centerIn: parent
                text: root.backend && root.backend.connected ? qsTr("场景接入中…") : qsTr("等待本地 Go 服务接入场景列表…")
                color: "#B8C8D8"
                font.pixelSize: 12
            }
        }
    }
}

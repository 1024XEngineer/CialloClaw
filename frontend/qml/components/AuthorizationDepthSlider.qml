import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

Item {
    id: root
    implicitHeight: 122

    property int depthIndex: 1

    readonly property var labels: [qsTr("仅预览"), qsTr("仅生成草稿"), qsTr("允许执行一次")]
    readonly property var keys: ["preview", "draft", "execute_once"]
    readonly property var descriptions: [
        qsTr("只看路径，不写入任何变化"),
        qsTr("生成可检查的草稿，不外发"),
        qsTr("执行一次并记录回滚点")
    ]

    readonly property string selectedKey: keys[Math.max(0, Math.min(2, depthIndex))]
    readonly property string selectedLabel: labels[Math.max(0, Math.min(2, depthIndex))]

    function keyToIndex(key) {
        const idx = keys.indexOf(key)
        return idx >= 0 ? idx : 1
    }

    Rectangle {
        anchors.fill: parent
        radius: 20
        color: Qt.rgba(0.09, 0.12, 0.20, 0.78)
        border.color: Qt.rgba(1, 1, 1, 0.08)
    }

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 14
        spacing: 8

        RowLayout {
            Layout.fillWidth: true
            Text {
                text: qsTr("授权深度")
                color: "#F5FAFF"
                font.pixelSize: 13
                font.bold: true
            }
            Item { Layout.fillWidth: true }
            Text {
                text: root.selectedLabel
                color: "#76D8FF"
                font.pixelSize: 11
                font.bold: true
            }
        }

        Slider {
            id: slider
            Layout.fillWidth: true
            from: 0
            to: 2
            stepSize: 1
            snapMode: Slider.SnapAlways
            value: root.depthIndex

            onMoved: root.depthIndex = Math.round(value)
            onValueChanged: root.depthIndex = Math.round(value)

            background: Rectangle {
                x: slider.leftPadding
                y: slider.topPadding + slider.availableHeight / 2 - height / 2
                width: slider.availableWidth
                height: 4
                radius: 2
                color: Qt.rgba(1, 1, 1, 0.08)

                Rectangle {
                    width: slider.visualPosition * parent.width
                    height: parent.height
                    radius: parent.radius
                    color: "#76D8FF"
                }
            }

            handle: Rectangle {
                x: slider.leftPadding + slider.visualPosition * (slider.availableWidth - width)
                y: slider.topPadding + slider.availableHeight / 2 - height / 2
                width: 22
                height: 22
                radius: 11
                color: "#F5FAFF"
                border.color: "#76D8FF"
                border.width: 2
            }
        }

        RowLayout {
            Layout.fillWidth: true
            spacing: 8

            Repeater {
                model: 3
                delegate: ColumnLayout {
                    required property int index
                    Layout.fillWidth: true

                    Text {
                        Layout.fillWidth: true
                        text: root.labels[index]
                        color: index === root.depthIndex ? "#FFFFFF" : "#B9CADD"
                        font.pixelSize: 11
                        font.bold: index === root.depthIndex
                        horizontalAlignment: Text.AlignHCenter
                    }

                    Text {
                        Layout.fillWidth: true
                        text: root.descriptions[index]
                        color: "#8FA6BF"
                        font.pixelSize: 10
                        wrapMode: Text.WordWrap
                        horizontalAlignment: Text.AlignHCenter
                    }
                }
            }
        }
    }
}

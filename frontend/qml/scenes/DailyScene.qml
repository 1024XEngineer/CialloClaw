import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

Item {
    id: root
    anchors.fill: parent

    property var sceneData: ({})
    property var situationData: ({})
    property var memoryData: ({})
    property bool draftVisible: false
    property var runtimeDraft: ({})

    signal primaryActionTriggered(string actionType, string actionKey)

    property var content: root.sceneData.content || ({})
    property var activeDraft: (root.runtimeDraft && root.runtimeDraft.title) ? root.runtimeDraft : (root.content.draft || ({}))

    function tint(type) {
        return type === "proceed" ? "#76D4D0" : "#76D8FF"
    }

    ColumnLayout {
        anchors.fill: parent
        spacing: 14

        RowLayout {
            Layout.fillWidth: true
            Layout.fillHeight: true
            spacing: 14

            Rectangle {
                Layout.fillWidth: true
                Layout.fillHeight: true
                radius: 22
                color: Qt.rgba(0.10, 0.14, 0.23, 0.72)
                border.color: Qt.rgba(1, 1, 1, 0.08)

                ColumnLayout {
                    anchors.fill: parent
                    anchors.margins: 16
                    spacing: 10

                    Text {
                        text: root.sceneData.title || qsTr("日报整理")
                        color: "#F6FAFF"
                        font.pixelSize: 20
                        font.bold: true
                    }

                    Text {
                        text: root.sceneData.storyLead || qsTr("按你平时习惯，这个时间点你通常会开始整理日报。")
                        color: "#DCE8F5"
                        font.pixelSize: 12
                        wrapMode: Text.WordWrap
                    }

                    Rectangle {
                        Layout.fillWidth: true
                        Layout.fillHeight: true
                        radius: 18
                        color: Qt.rgba(1, 1, 1, 0.035)
                        border.color: Qt.rgba(1, 1, 1, 0.05)

                        ColumnLayout {
                            anchors.fill: parent
                            anchors.margins: 14
                            spacing: 10

                            Text {
                                text: qsTr("今日已完成事项")
                                color: "#FFFFFF"
                                font.pixelSize: 13
                                font.bold: true
                            }

                            Repeater {
                                model: root.content.completedItems || []

                                delegate: Text {
                                    required property var modelData
                                    text: "• " + modelData
                                    color: "#DCE8F5"
                                    font.pixelSize: 11
                                    wrapMode: Text.WordWrap
                                }
                            }

                            Rectangle {
                                Layout.fillWidth: true
                                Layout.preferredHeight: 1
                                color: Qt.rgba(1, 1, 1, 0.08)
                            }

                            Text {
                                text: qsTr("待跟进项")
                                color: "#9DB5CC"
                                font.pixelSize: 11
                            }

                            Repeater {
                                model: root.content.followUpItems || []

                                delegate: Text {
                                    required property var modelData
                                    text: "• " + modelData
                                    color: "#BFD1E2"
                                    font.pixelSize: 11
                                    wrapMode: Text.WordWrap
                                }
                            }
                        }
                    }
                }
            }

            Rectangle {
                Layout.preferredWidth: 380
                Layout.fillHeight: true
                radius: 22
                color: Qt.rgba(0.09, 0.14, 0.22, 0.74)
                border.color: Qt.rgba(1, 1, 1, 0.08)

                states: [
                    State {
                        name: "draft-open"
                        when: root.draftVisible
                        PropertyChanges { target: draftPanel; opacity: 1.0; y: 0 }
                    },
                    State {
                        name: "draft-closed"
                        when: !root.draftVisible
                        PropertyChanges { target: draftPanel; opacity: 0.42; y: 6 }
                    }
                ]

                transitions: Transition {
                    NumberAnimation { properties: "opacity,y"; duration: 220; easing.type: Easing.OutCubic }
                }

                ColumnLayout {
                    id: draftPanel
                    anchors.fill: parent
                    anchors.margins: 16
                    spacing: 10

                    Text {
                        text: qsTr("日报草稿")
                        color: "#F6FAFF"
                        font.pixelSize: 14
                        font.bold: true
                    }

                    Text {
                        text: root.activeDraft.status || qsTr("推进一步后，这里会展开草稿")
                        color: root.draftVisible ? "#76D4D0" : "#9FB3C7"
                        font.pixelSize: 12
                        font.bold: root.draftVisible
                    }

                    Rectangle {
                        Layout.fillWidth: true
                        Layout.fillHeight: true
                        radius: 18
                        color: Qt.rgba(1, 1, 1, 0.035)
                        border.color: Qt.rgba(1, 1, 1, 0.05)

                        ColumnLayout {
                            anchors.fill: parent
                            anchors.margins: 14
                            spacing: 8

                            Text {
                                text: root.activeDraft.title || qsTr("草稿结构")
                                color: "#FFFFFF"
                                font.pixelSize: 13
                                font.bold: true
                            }

                            Repeater {
                                model: root.activeDraft.sections || []

                                delegate: Rectangle {
                                    required property var modelData
                                    Layout.fillWidth: true
                                    radius: 14
                                    color: Qt.rgba(0.15, 0.20, 0.28, 0.52)
                                    border.color: Qt.rgba(1, 1, 1, 0.05)

                                    ColumnLayout {
                                        anchors.fill: parent
                                        anchors.margins: 10
                                        spacing: 4

                                        Text {
                                            text: modelData.header || ""
                                            color: "#76D4D0"
                                            font.pixelSize: 11
                                            font.bold: true
                                        }

                                        Repeater {
                                            model: modelData.bullets || []

                                            delegate: Text {
                                                required property var modelData
                                                text: "• " + modelData
                                                color: "#DCE8F5"
                                                font.pixelSize: 10
                                                wrapMode: Text.WordWrap
                                            }
                                        }
                                    }
                                }
                            }

                            Text {
                                text: root.activeDraft.nextStep || qsTr("草稿先按你的习惯排好，后面只要补细节。")
                                color: "#B4C8DB"
                                font.pixelSize: 10
                                wrapMode: Text.WordWrap
                            }
                        }
                    }
                }
            }
        }

        Rectangle {
            Layout.fillWidth: true
            Layout.preferredHeight: 96
            radius: 22
            color: Qt.rgba(0.09, 0.14, 0.22, 0.74)
            border.color: Qt.rgba(1, 1, 1, 0.08)

            RowLayout {
                anchors.fill: parent
                anchors.margins: 14
                spacing: 14

                ColumnLayout {
                    Layout.fillWidth: true
                    spacing: 4
                    Text {
                        text: qsTr("按你的习惯")
                        color: "#9DB5CC"
                        font.pixelSize: 11
                    }
                    Text {
                        text: (root.content.habitChecks || []).join(" · ")
                        color: "#DCE8F5"
                        font.pixelSize: 11
                        wrapMode: Text.WordWrap
                    }
                }

                Button {
                    text: root.sceneData.primaryAction ? root.sceneData.primaryAction.label : qsTr("推进一步")
                    onClicked: {
                        const action = root.sceneData.primaryAction || ({})
                        if (action.key) {
                            root.primaryActionTriggered(action.type, action.key)
                        }
                    }
                    background: Rectangle {
                        radius: 18
                        color: root.tint(root.sceneData.primaryAction ? root.sceneData.primaryAction.type : "proceed")
                    }
                    contentItem: Text {
                        text: parent.text
                        color: "#08111E"
                        font.pixelSize: 13
                        font.bold: true
                        horizontalAlignment: Text.AlignHCenter
                        verticalAlignment: Text.AlignVCenter
                    }
                }
            }
        }
    }
}

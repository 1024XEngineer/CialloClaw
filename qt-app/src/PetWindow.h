#pragma once

#include <QWidget>
#include <QMenu>
#include <QSystemTrayIcon>
#include <QNetworkAccessManager>
#include <QNetworkReply>
#include <QPoint>
#include <QMouseEvent>
#include <QPixmap>

class PetWindow : public QWidget {
    Q_OBJECT

public:
    explicit PetWindow(QWidget *parent = nullptr);
    ~PetWindow();

signals:
    void openChatRequested();
    void openSettingsRequested();
    void hideRequested();
    void quitRequested();

protected:
    void mousePressEvent(QMouseEvent *event) override;
    void mouseMoveEvent(QMouseEvent *event) override;
    void mouseReleaseEvent(QMouseEvent *event) override;
    void contextMenuEvent(QContextMenuEvent *event) override;
    void paintEvent(QPaintEvent *event) override;

private slots:
    void onTrayActivated(QSystemTrayIcon::ActivationReason reason);
    void onPetStatusReply();

private:
    void setupMask();
    void setupTray();
    void fetchPetStatus();
    void showContextMenu(const QPoint &pos);
    void rebuildSprite();

    QMenu *contextMenu_;
    QSystemTrayIcon *trayIcon_;
    QMenu *trayMenu_;
    QNetworkAccessManager *networkManager_;
    QPoint dragStartPosition_;
    bool isDragging_;
    QPixmap petPixmap_;
};

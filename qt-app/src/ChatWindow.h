#pragma once

#include <QQuickView>
#include <QNetworkAccessManager>
#include <QStringList>

class QCloseEvent;

class ChatWindow : public QQuickView {
    Q_OBJECT
    Q_PROPERTY(QString title READ title NOTIFY titleChanged)
    Q_PROPERTY(QStringList messages READ messages NOTIFY messagesChanged)

public:
    explicit ChatWindow(QWindow *parent = nullptr);
    ~ChatWindow() override;

    QString title() const { return title_; }
    QStringList messages() const { return messages_; }

public slots:
    void sendMessage(const QString &text);
    void startWindowMove();
    void hideWindow();

signals:
    void titleChanged();
    void messagesChanged();

protected:
    void closeEvent(QCloseEvent *event) override;

private slots:
    void onChatInitReply();

private:
    void fetchChatInit();

    QNetworkAccessManager *networkManager_;
    QString title_;
    QStringList messages_;
};

#pragma once

#include <QWidget>

class SettingsWindow : public QWidget {
    Q_OBJECT

public:
    explicit SettingsWindow(QWidget *parent = nullptr);
    ~SettingsWindow() override;

protected:
    void closeEvent(QCloseEvent *event) override;
};

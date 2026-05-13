# Quick Start

## Download and Launch

### Download

You can choose from two versions:

| Version | Audience | Description |
| ------- | -------- | ----------- |
| Latest | General users | Recommended download, ready to use after installation |
| Preview | Developers / early adopters | Start from source, get the latest features |

### Download options:

- [Download the latest version (GitHub Releases, latest currently points to V0.3.0)](https://github.com/1024XEngineer/CialloClaw/releases/latest)
- [Download the preview version (source repository)](https://github.com/1024XEngineer/CialloClaw)

------

## Install the Latest Version

Click "Download the latest version" and follow the installation guide.

Once installed, launch CialloClaw.

If it starts successfully, you will see the floating ball on your desktop.

------

## Install the Preview Version

The preview version is suitable for users who want to try the latest features, or who want to run CialloClaw from source.

Before you start, please prepare:

```text
Node.js
pnpm
Git
Rust / Tauri development environment
```

### Step 1: Get the Source Code

You can get the preview source code in two ways.

#### Option 1: Clone with Git

Open a terminal and run:

```cmd
git clone https://github.com/1024XEngineer/CialloClaw.git
```

Then enter the project folder:

```cmd
cd CialloClaw
```

#### Option 2: Download ZIP

Go to the GitHub repository page and click:

```text
Code → Download ZIP
```

After downloading, extract the ZIP file.

Then enter the extracted `CialloClaw` folder.

------

### Step 2: Install Dependencies

Right-click in the root directory of the `CialloClaw` folder and open a terminal.

Run:

```cmd
pnpm install
```

Wait for the dependencies to finish installing.

------

### Step 3: Start the Local Service

In the same terminal, continue running:

```cmd
pnpm dev:service
```

Do not close this terminal after running.
It needs to stay running.

------

### Step 4: Launch the Desktop App

Go back to the `CialloClaw` folder root directory.

Right-click again and open a second terminal.

Run:

```cmd
pnpm --dir apps/desktop exec tauri dev
```

After a moment, CialloClaw will launch automatically.

If it starts successfully, you will see the floating ball on your desktop.

------

### Preview Version Startup Summary

```text
1. Get the source code
2. Enter the CialloClaw root directory
3. Run pnpm install
4. First terminal: run pnpm dev:service
5. Keep the first terminal running
6. Second terminal: run pnpm --dir apps/desktop exec tauri dev
7. Wait for the desktop app to launch
```

The first time you start it, CialloClaw will automatically enter the onboarding guide.

Follow the guide to get a basic understanding of the floating ball, voice, workspace, and control panel.

------

## Configure a Model

CialloClaw needs a model configured before it can reply properly.

### Open the Control Panel

Find the CialloClaw icon in the system tray.

Right-click the icon and select:

```text
Control Panel
```

Once in the control panel, click on the left side:

```text
Models & Security
```

### Fill in the Model Information

Fill in the model configuration on the right side.

Note:

```text
Provider can be named freely.
The remaining fields require correct model service information.
```

After filling in, click:

```text
Test Connection
```

If the test succeeds, then click:

```text
Save Settings
```

At this point, the model configuration is complete.

------

## Try It Out

Once configured, you can try three simple actions.

### Send a Message

Click the input field below the floating ball and type:

```text
Hello, introduce what you can do
```

After sending, CialloClaw will reply in a bubble.

------

### Long Press the Floating Ball

Long press the floating ball and speak:

```text
Summarise today's news
```

CialloClaw will process your voice input and give a reply.

It works best in a quiet environment.

------

### Drag and Drop a File

Drag any file onto the floating ball.

CialloClaw will read the file content and you can interact with it.

------

## Common Actions Overview

### Summarise a Webpage

```text
Open a page → long press the floating ball → say "Summarise this page for me"
```

### Translate or Explain Text

```text
Select text → click the floating ball → ask it to explain or translate
```

### Analyse a File

```text
Drag in a file → add a request → send
```

For example:

```text
Summarise the key points of this document for me
```

### Explain an Error

```text
Select the error → click the floating ball → ask it to analyse the cause
```

### Draft Content

You can ask CialloClaw to draft:

```text
Daily report draft
Email draft
Meeting checklist
Presentation outline
Next-step plan
```

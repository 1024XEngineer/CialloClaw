# Frequently Asked Questions

## The workspace is blank, what can I do?

Right-click inside the workspace window and select Refresh.

If it still does not recover, you can exit CialloClaw and restart it.

------

## The floating ball is missing

It may be snapped to the edge of the screen.

Check around the edges of your screen first.
You can also right-click the CialloClaw icon in the system tray and click:

```text
Show Floating Ball
```

------

## No response after typing

Please check the following:

```text
Whether a model has been configured
Whether the test connection was successful
Whether the network is working
```

If you are using the preview version, also confirm that the first terminal's service is still running:

```cmd
pnpm dev:service
```

------

## Voice input is not working

Please check:

```text
Whether the system allows CialloClaw to use the microphone
Whether the current device has an available microphone
Whether the model configuration is correct
```

------

## The preview version fails to start

Check the following in order:

```text
Is pnpm installed
Is the Rust / Tauri development environment set up
Are commands executed in the project root directory
Did pnpm install succeed
Is the first terminal still running
Is the second terminal command entered correctly
```

------

## What is the difference between the latest and preview versions?

**Latest version** is more stable, suitable for general users.

**Preview version** receives updates faster, suitable for developers and early adopters. It may contain unfinished or unstable features.

------

## Is CialloClaw free?

CialloClaw itself is open source and free.

If you use third-party model services, model API fees may apply. The cost depends on the model and service provider you configure.

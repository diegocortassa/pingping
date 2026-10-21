# AGENTS.md

## General Instructions for the agents

** Always ignore files in the scratch directory **

Always look for answers online, be sure you have references before coming back with answers. If you can't find an answer, write that.

Make sure to be concise with answers always, don't soft talk sweet talk or anything, be accurate, to the point and analytic.

Use semantic versioning.

When adding comments or writing documentation never use unicode characters like ->,  that are not typeable with a simple keyboard, e.g. avoid emojis and special symbols like em-dashes, arrows and so on use - normal dashes or -> for arrows and so on.

We want the code to be simple, minimal, correct, secure, readable and follow best practices, with minimal but meaningfull comments

For GUI applications use Fyne

## code style
- Always strive for coincise, simple solutions.
- If a problem can be solved in a simpler way, propose it.

## Introduction
PingPing is a network monitoring tool designed to provide real-time insights into the status and performance of an user's network connection. It aims to be simple, minimal, and efficient while offering accurate and actionable information. It allows the user to configure one or more addresses to ping. It keeps a log of the ping results for historical analysis and troubleshooting. It plots a graphical representation of the network performance over time for each monitored address in real time. The displayed graphics length is configurable and defaults to 1 hour. The history is logged so if the user wants to review past network performance, they can do so easily. Logs are stored persistently for future reference and rotated every 24 hours.

## Technical Details
-It is written in Go and portable to Linux, Windows and MacOS.
-It uses Fyne for the GUI, ensuring a consistent look and feel across different platforms.
-when the main windows is closed it minimizes to the system tray and keeps monitoring.
-It supports configurable ping intervals and timeout settings for each monitored address.
-It provides real-time graphical representation of network performance for each monitored address.
-It maintains a persistent log of ping results, rotated every 24 hours.
-It does not need to be installed, it is fully portable, just unzip the archibe and run.
-It use the icons provided in the `assets/icons` directory for the system tray and application window. The are in svg format, convert them to the required format if necessary using ImageMagick convert tool.

## Notes
- Before starting write a plan and outline in PLAN.md
- Keep tracks of todo and done, in TODO.md
- Keep a small diary of changes and observations in DIARY.md
- Keep the README.md up to date with relevant information about the project. Small presentation, user instruction on how to use, developer instruction on how to compile.

## Maintainability

Long term maintainability is a core priority. If you add new functionality, first check if there is shared logic that can be extracted to a separate module. Duplicate logic across multiple files is a code smell and should be avoided. Don't be afraid to change existing code. Don't take shortcuts by just adding local logic to solve a problem.

## Conventions
- for json fields we use camelCase unless we are dealing with field from the dcv command, in that case we keep the dcv convetion (snake_case)
- for database fields name, database tables names we use snake_case
- for Go structs fields we use PascalCase for exported fields, camelCase for unexported fields.
- api methods live under internal/v1/{PATH} to mirror the endpoint path

## Building and Running

The project uses a `Makefile` to simplify common tasks:

*   **Build the application (frontend + binary):**
    ```bash
    make build
    ```
    This builds the frontend and creates a Linux executable in the `dist/` directory.

*   **Run tests:**
    ```bash
    make test
    ```

*   **Run quality control checks:**
    ```bash
    make audit
    ```
    Runs `go mod tidy -diff`, `go mod verify`, `gofmt`, `go vet`, `staticcheck`, and `govulncheck`.

*   **Run the application:**
    ```bash
    make run
    ```

*   **Clean the build artifacts:**
    ```bash
    make clean
    ```

*   **Build Debian package:**
    ```bash
    make deb
    ```

*   **Build RPM package:**
    ```bash
    make rpm
    ```

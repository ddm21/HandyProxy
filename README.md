# Handy Voice-to-Text LLM Proxy

A native, lightweight background Windows utility that intercepts raw dictation from [Handy.computer](https://handy.computer), automatically cleans it up using Groq's extremely fast LLM inference, and native-pastes the beautifully formatted text into your active window.

## Why I Created This
Handy is a fantastic, privacy-focused voice-to-text tool that runs locally. However, its raw voice-to-text output lacks the intelligent formatting, bullet points, and grammar correction of cloud tools like WhisperFlow. 

This proxy solves that by acting as a native bridge: it waits for Handy to finish transcribing, intercepts the raw text, uses a blazing fast LLM via Groq to remove filler words and add proper formatting, and then instantly simulates a `Ctrl+V` keystroke to paste it for you. 

*Written entirely in Go, this proxy uses 0 background CPU and very little RAM, replacing bulky Python environments with a single, standalone executable.*

## Setup Instructions

### 1. Configure Handy
Open the Handy.computer application settings and configure the following under the **Output** section:
- **Paste Method:** `None`
- **Clipboard Handling:** `Copy to Clipboard`

### 2. Get a Free Groq API Key
Groq offers insanely fast inference using Llama 3 models, making it perfect for real-time dictation cleanup.
1. Go to [Groq Console](https://console.groq.com/keys)
2. Sign in and click **"Create API Key"**
3. Copy your API key.

### 3. Using the App
1. Download or compile the `HandyProxy.exe`.
2. Double-click to open the Settings GUI.
3. Paste your Groq API key.
4. (Optional) Adjust the model if desired. The default is `llama-3.1-8b-instant`.
5. Click **Save Settings**.
6. The app will minimize to your System Tray and run silently in the background.

When you dictate with Handy, the proxy intercepts the clipboard text, formats it via Groq, and pastes it into your window in milliseconds.

*Note: The proxy uses the Windows API to detect if the text came from the target app (e.g., 'handy'). It will not intercept text you copy manually from WhatsApp, Notepad, etc., unless configured otherwise.*

---

## How to Build from Source (For Developers)

If you want to modify the code or build the `.exe` yourself, follow these steps:

1. **Install Go:** Download and install Go from [go.dev/dl](https://go.dev/dl/).

2. **Clone the repository:**
   ```bash
   git clone https://github.com/ddm21/HandyProxy.git
   cd HandyProxy
   ```

3. **Install Dependencies and Tools:**
   ```powershell
   go mod tidy
   go get github.com/lxn/walk
   go get github.com/atotto/clipboard
   go install github.com/akavel/rsrc@latest
   ```

4. **Compile the UI Manifest:**
   This embeds the manifest so the UI uses modern Windows 10/11 visual styles instead of legacy styles.
   ```powershell
   rsrc -manifest main.exe.manifest -o rsrc.syso
   ```
   *(Note: Ensure your `$(go env GOPATH)\bin` is added to your system PATH to run `rsrc` directly).*

5. **Compile into a standalone hidden `.exe`:**
   ```powershell
   go build -ldflags="-H windowsgui" -o HandyProxy.exe
   ```
   *Your compiled executable `HandyProxy.exe` will be ready in the project folder!*

## Logs
Your daily transcription logs (original vs. formatted text) and configuration are automatically saved to `C:\Users\<Your_Username>\HandyProxy\`.

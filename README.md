# Handy Voice-to-Text LLM Proxy

A background Windows utility that intercepts raw dictation from [Handy.computer](https://handy.computer), automatically cleans it up using Gemini's LLM, and pastes the beautifully formatted text into your active window.

## Why I Created This
Handy is a fantastic, privacy-focused voice-to-text tool that runs locally. However, its raw voice-to-text output lacks the intelligent formatting, bullet points, and grammar correction of cloud tools like WhisperFlow. 

This proxy solves that by acting as a bridge: it waits for Handy to finish transcribing, intercepts the raw text, uses a very cheap and fast LLM (Gemini Flash Lite) to remove filler words and add proper formatting, and then instantly pastes it for you.

## Setup Instructions

### 1. Configure Handy
Open the Handy.computer application settings and configure the following under the **Output** section:
- **Paste Method:** `None`
- **Clipboard Handling:** `Copy to Clipboard`

### 2. Get a Free Gemini API Key
The Google Gemini API has an incredibly generous free tier that is perfect for this.
1. Go to [Google AI Studio](https://aistudio.google.com/)
2. Sign in and click **"Get API key"**
3. Create an API key and copy it.

### 3. Using the App
1. Download or compile the `handy_proxy_gui.exe`.
2. Double-click to open the Settings GUI.
3. Paste your Gemini API key.
4. (Optional) Adjust the system prompt or model if desired. The default is `gemini-2.5-flash-lite`.
5. Click **Save Settings**.
6. Close the window! The app will minimize to your System Tray (green square icon) and run silently in the background.

When you dictate with Handy, the tray icon will turn **Orange** while it formats your text, and snap back to **Green** once it pastes it into your window.

*Note: The proxy is smart—it uses the Windows API to detect if the text came from Handy. It will not intercept text you copy manually from WhatsApp, Notepad, etc.*

---

## How to Build from Source (For Developers)

If you want to modify the code or build the `.exe` yourself, follow these steps:

1. **Clone the repository:**
   ```bash
   git clone https://github.com/ddm21/HandyProxy.git
   cd HandyProxy
   ```

2. **Create a Virtual Environment:**
   ```bash
   python -m venv venv
   source venv/Scripts/activate  # On Windows Git Bash
   ```

3. **Install Dependencies:**
   ```bash
   pip install -r requirements.txt
   ```

4. **Run the script directly:**
   ```bash
   python handy_proxy_gui.py
   ```

5. **Compile into a standalone `.exe`:**
   ```bash
   python -m PyInstaller --noconsole --onefile --clean handy_proxy_gui.py
   ```
   *Your compiled executable will be waiting for you in the `dist` folder!*

## Logs
Your daily transcription logs (original vs. formatted text) and configuration are automatically saved to `C:\Users\<Your_Username>\HandyProxy\`.

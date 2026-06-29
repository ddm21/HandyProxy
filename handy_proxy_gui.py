import os
import json
import time
from datetime import datetime
import threading
import keyboard
import pyperclip
import tkinter as tk
from tkinter import messagebox
from google import genai
import pystray
from PIL import Image, ImageDraw
import ctypes
from ctypes import wintypes

# --- Directory and File Setup ---
BASE_DIR = os.path.expanduser(r"~\HandyProxy")
LOGS_DIR = os.path.join(BASE_DIR, "logs")
CONFIG_FILE = os.path.join(BASE_DIR, "config.json")

# Ensure directories exist upon startup
os.makedirs(LOGS_DIR, exist_ok=True)

DEFAULT_PROMPT = """You are an expert transcriber and developer assistant. Your job is to format raw voice-to-text dictation into clean, highly readable text.

Instructions:
1. Remove filler words (um, uh, like, you know) and correct grammar/punctuation.
2. Detect natural list structures (e.g., "number one", "secondly", "bullet point") and format them into proper numbered or bulleted lists with line breaks.
3. Format coding terms, file names (e.g., script.py, index.html), and programming languages clearly. Use markdown code backticks (`code`) for variables, file names, and technical terms if appropriate.
4. Add proper paragraph breaks for readability.
5. DO NOT add any conversational padding (like "Here is the text:"). Output ONLY the final formatted text.

Raw Text: {raw_text}"""

default_config = {
    "api_key": "",
    "model": "gemini-2.5-flash-lite",
    "target_app": "handy",
    "prompt": DEFAULT_PROMPT
}

def load_config():
    if os.path.exists(CONFIG_FILE):
        try:
            with open(CONFIG_FILE, "r", encoding="utf-8") as f:
                return json.load(f)
        except:
            pass
    return default_config.copy()

def save_config(config_data):
    with open(CONFIG_FILE, "w", encoding="utf-8") as f:
        json.dump(config_data, f, indent=4)

config = load_config()

# --- Logging System ---
def log_transcription(raw_text, clean_text):
    now = datetime.now()
    date_str = now.strftime("%Y-%m-%d")
    timestamp_str = now.strftime("%Y-%m-%d-%H:%M:%S")
    
    log_file = os.path.join(LOGS_DIR, f"{date_str}.txt")
    
    log_entry = f"Time: {timestamp_str}\n\n"
    log_entry += f"[ORIGINAL TEXT]\n{raw_text}\n\n"
    log_entry += f"[GEMINI FORMATTED TEXT]\n{clean_text}\n"
    log_entry += f"--------------------------------------------------\n\n"
    
    try:
        with open(log_file, "a", encoding="utf-8") as f:
            f.write(log_entry)
    except Exception as e:
        print(f"Failed to write log: {e}")

# --- Clipboard Owner API ---
def get_clipboard_owner_process():
    user32 = ctypes.windll.user32
    kernel32 = ctypes.windll.kernel32

    hwnd = user32.GetClipboardOwner()
    if not hwnd:
        return None

    pid = wintypes.DWORD()
    user32.GetWindowThreadProcessId(hwnd, ctypes.byref(pid))

    if pid.value:
        h_process = kernel32.OpenProcess(0x0410, False, pid.value)
        if h_process:
            buffer_len = wintypes.DWORD(260)
            buffer = ctypes.create_unicode_buffer(buffer_len.value)
            if kernel32.QueryFullProcessImageNameW(h_process, 0, buffer, ctypes.byref(buffer_len)):
                kernel32.CloseHandle(h_process)
                return buffer.value
            kernel32.CloseHandle(h_process)
    return ""

# --- LLM and Monitor Logic ---
def format_text_with_llm(raw_text):
    api_key = config.get("api_key", "").strip()
    if not api_key:
        return raw_text
    
    try:
        client = genai.Client(api_key=api_key)
        prompt_template = config.get("prompt", DEFAULT_PROMPT)
        
        if "{raw_text}" in prompt_template:
            final_prompt = prompt_template.replace("{raw_text}", raw_text)
        else:
            final_prompt = prompt_template + f"\n\nRaw Text: {raw_text}"
            
        model_name = config.get("model", "gemini-2.5-flash-lite")
        
        response = client.models.generate_content(
            model=model_name,
            contents=final_prompt,
        )
        return response.text.strip()
    except Exception as e:
        print(f"LLM Error: {e}")
        return raw_text

def monitor_and_paste():
    last_clipboard_content = pyperclip.paste()
    while True:
        try:
            current_clipboard = pyperclip.paste()
            if current_clipboard != last_clipboard_content and current_clipboard != "":
                
                owner_process = get_clipboard_owner_process()
                target_app = config.get("target_app", "").lower().strip()
                
                # If target_app is defined and the owner doesn't match, IGNORE IT
                if target_app and owner_process and target_app not in owner_process.lower():
                    last_clipboard_content = current_clipboard
                    time.sleep(0.3)
                    continue

                if tray_icon:
                    tray_icon.icon = create_image("orange")
                
                clean_text = format_text_with_llm(current_clipboard)
                
                # Log the transcription to the daily file
                log_transcription(current_clipboard, clean_text)
                
                # Prevent looping by updating our tracker first
                last_clipboard_content = clean_text 
                pyperclip.copy(clean_text)
                
                # Simulate Paste
                time.sleep(0.1)
                keyboard.send('ctrl+v')
                
                if tray_icon:
                    tray_icon.icon = create_image("green")
                
        except Exception:
            pass
        time.sleep(0.3)

# Start background clipboard monitor
monitor_thread = threading.Thread(target=monitor_and_paste, daemon=True)
monitor_thread.start()

# --- GUI Logic ---
root = tk.Tk()
root.title("Handy Voice-to-Text Proxy")
root.geometry("600x630")

tk.Label(root, text="Gemini API Key:", font=("Arial", 10, "bold")).pack(pady=(5,0))
api_key_entry = tk.Entry(root, width=60, show="*")
api_key_entry.insert(0, config.get("api_key", ""))
api_key_entry.pack()

tk.Label(root, text="Model Name:", font=("Arial", 10, "bold")).pack(pady=(5,0))
model_entry = tk.Entry(root, width=60)
model_entry.insert(0, config.get("model", "gemini-2.5-flash-lite"))
model_entry.pack()

tk.Label(root, text="Target App Name Filter (e.g., 'handy' - Leave blank for all):", font=("Arial", 10, "bold")).pack(pady=(5,0))
target_app_entry = tk.Entry(root, width=60)
target_app_entry.insert(0, config.get("target_app", "handy"))
target_app_entry.pack()

tk.Label(root, text="System Prompt (must include {raw_text}):", font=("Arial", 10, "bold")).pack(pady=(5,0))
prompt_text = tk.Text(root, width=70, height=16)
prompt_text.insert("1.0", config.get("prompt", DEFAULT_PROMPT))
prompt_text.pack()

def save_and_apply():
    config["api_key"] = api_key_entry.get().strip()
    config["model"] = model_entry.get().strip()
    config["target_app"] = target_app_entry.get().strip()
    config["prompt"] = prompt_text.get("1.0", "end-1c")
    save_config(config)
    messagebox.showinfo("Saved", "Settings saved successfully!\nThey are immediately active in the background.")

# Buttons frame
btn_frame = tk.Frame(root)
btn_frame.pack(pady=10)

tk.Button(btn_frame, text="Save Settings", command=save_and_apply, font=("Arial", 10, "bold"), bg="#4CAF50", fg="white").pack(side=tk.LEFT, padx=5)

def open_logs_folder():
    os.startfile(LOGS_DIR)

tk.Button(btn_frame, text="Open Logs Folder", command=open_logs_folder, font=("Arial", 10)).pack(side=tk.LEFT, padx=5)

# --- System Tray Logic ---
def create_image(color="green"):
    if color == "orange":
        rgb = (255, 165, 0)
    else:
        rgb = (0, 128, 0)
    image = Image.new('RGB', (64, 64), color = rgb)
    draw = ImageDraw.Draw(image)
    draw.rectangle([16, 16, 48, 48], fill=(255, 255, 255))
    return image

tray_icon = None

def quit_action(icon, item):
    icon.stop()
    root.quit()

def show_action(icon, item):
    icon.stop()
    root.after(0, root.deiconify)

def hide_window():
    root.withdraw()
    global tray_icon
    menu = pystray.Menu(
        pystray.MenuItem('Show Settings', show_action),
        pystray.MenuItem('Quit', quit_action)
    )
    tray_icon = pystray.Icon("HandyProxy", create_image(), "Handy Proxy", menu)
    threading.Thread(target=tray_icon.run, daemon=True).start()

root.protocol('WM_DELETE_WINDOW', hide_window)
root.mainloop()

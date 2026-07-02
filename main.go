package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/atotto/clipboard"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

var (
	user32                         = syscall.NewLazyDLL("user32.dll")
	kernel32                       = syscall.NewLazyDLL("kernel32.dll")
	procGetClipboardOwner          = user32.NewProc("GetClipboardOwner")
	procGetWindowThreadProcessId   = user32.NewProc("GetWindowThreadProcessId")
	procOpenProcess                = kernel32.NewProc("OpenProcess")
	procQueryFullProcessImageNameW = kernel32.NewProc("QueryFullProcessImageNameW")
	procCloseHandle                = kernel32.NewProc("CloseHandle")
	procSendInput                  = user32.NewProc("SendInput")
	procCreateMutexW               = kernel32.NewProc("CreateMutexW")
	procGetLastError               = kernel32.NewProc("GetLastError")
	procFindWindowW                = user32.NewProc("FindWindowW")
	procShowWindow                 = user32.NewProc("ShowWindow")
	procSetForegroundWindow        = user32.NewProc("SetForegroundWindow")
)

const (
	PROCESS_QUERY_INFORMATION         = 0x0400
	PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
	ERROR_ALREADY_EXISTS              = 183
	SW_RESTORE                        = 9
)

const defaultPrompt = `You are a text formatting assistant for raw voice-to-text dictation. Your job is to clean up the text by applying casual formatting and fixing grammar, while preserving the natural conversational tone.

Instructions:
1. Fix punctuation and correct obvious grammar mistakes to make the text clear and readable.
2. Remove filler words ("um", "uh", "like") and stuttering.
3. Keep the natural, casual conversational tone. Do NOT rewrite sentences to sound overly formal, robotic, or academic.
4. Preserve the user's original phrasing and structure as much as possible; only change words if the sentence is broken or confusing.
5. Add paragraph breaks only when the topic clearly changes.
6. ONLY output the final formatted text. Do NOT add any greetings or conversational padding to your response.

Raw Text: %s`

type Config struct {
	ApiKey                string `json:"api_key"`
	Model                 string `json:"model"`
	TargetApp             string `json:"target_app"`
	SystemPrompt          string `json:"system_prompt"`
	RateRemainingRequests string `json:"rate_remaining_requests"`
	RateRemainingTokens   string `json:"rate_remaining_tokens"`
	RateLimitRequests     string `json:"rate_limit_requests"`
	RateLimitTokens       string `json:"rate_limit_tokens"`
	RateResetTokens       string `json:"rate_reset_tokens"`
}

var (
	config  Config
	baseDir string
	logsDir string
	cfgFile string
	mw      *walk.MainWindow
	ni      *walk.NotifyIcon
	
	// Rate limit UI bindings
	lblReqRem, lblTokRem, lblTokRes *walk.Label
)

func initPaths() {
	home, _ := os.UserHomeDir()
	baseDir = filepath.Join(home, "HandyProxy")
	logsDir = filepath.Join(baseDir, "logs")
	cfgFile = filepath.Join(baseDir, "config.json")
	os.MkdirAll(logsDir, os.ModePerm)
}

func loadConfig() {
	config = Config{
		Model: "llama-3.1-8b-instant",
		SystemPrompt: defaultPrompt,
	}
	b, err := os.ReadFile(cfgFile)
	if err == nil {
		json.Unmarshal(b, &config)
		if config.SystemPrompt == "" {
			config.SystemPrompt = defaultPrompt
		}
	}
}

func saveConfig() {
	b, _ := json.MarshalIndent(config, "", "  ")
	os.WriteFile(cfgFile, b, 0644)
}

func logTranscription(raw, clean string) {
	now := time.Now()
	dateStr := now.Format("2006-01-02")
	timeStr := now.Format("2006-01-02-15:04:05")

	logPath := filepath.Join(logsDir, fmt.Sprintf("%s.txt", dateStr))

	entry := fmt.Sprintf("Time: %s\n\n[ORIGINAL TEXT]\n%s\n\n[GROQ FORMATTED TEXT]\n%s\n--------------------------------------------------\n\n", timeStr, raw, clean)

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		f.WriteString(entry)
		f.Close()
	}
}

func getClipboardOwnerProcess() string {
	hwnd, _, _ := procGetClipboardOwner.Call()
	if hwnd == 0 {
		return ""
	}

	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return ""
	}

	hProcess, _, _ := procOpenProcess.Call(PROCESS_QUERY_INFORMATION|PROCESS_QUERY_LIMITED_INFORMATION, 0, uintptr(pid))
	if hProcess == 0 {
		return ""
	}
	defer procCloseHandle.Call(hProcess)

	var size uint32 = 260
	buf := make([]uint16, size)
	ret, _, _ := procQueryFullProcessImageNameW.Call(hProcess, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if ret == 0 {
		return ""
	}

	return syscall.UTF16ToString(buf)
}

func refreshGroqLimits() {
	apiKey := strings.TrimSpace(config.ApiKey)
	if apiKey == "" {
		return
	}
	body := `{"model":"` + config.Model + `","messages":[{"role":"user","content":"."}],"max_tokens":1}`
	req, _ := http.NewRequest("POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		defer resp.Body.Close()
		reqRem := resp.Header.Get("x-ratelimit-remaining-requests")
		tokRem := resp.Header.Get("x-ratelimit-remaining-tokens")
		tokRes := resp.Header.Get("x-ratelimit-reset-tokens")
		if reqRem != "" {
			config.RateRemainingRequests = reqRem
			config.RateRemainingTokens = tokRem
			config.RateResetTokens = tokRes
			saveConfig()
			if mw != nil {
				mw.Synchronize(func() {
					if lblReqRem != nil {
						lblReqRem.SetText(reqRem)
						lblTokRem.SetText(tokRem)
						lblTokRes.SetText(tokRes)
					}
				})
			}
		}
	}
}

type GroqRequest struct {
	Model    string        `json:"model"`
	Messages []GroqMessage `json:"messages"`
}

type GroqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GroqResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func formatTextWithLLM(rawText string) string {
	apiKey := strings.TrimSpace(config.ApiKey)
	if apiKey == "" {
		return rawText
	}

	prompt := strings.Replace(config.SystemPrompt, "%s", rawText, 1)

	reqBody := GroqRequest{
		Model: config.Model,
		Messages: []GroqMessage{
			{Role: "user", Content: prompt},
		},
	}

	b, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewBuffer(b))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Groq API error:", err)
		return rawText
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		log.Println("Groq API returned status:", resp.StatusCode, string(body))
		return rawText
	}

	reqRem := resp.Header.Get("x-ratelimit-remaining-requests")
	tokRem := resp.Header.Get("x-ratelimit-remaining-tokens")
	reqLim := resp.Header.Get("x-ratelimit-limit-requests")
	tokLim := resp.Header.Get("x-ratelimit-limit-tokens")
	tokRes := resp.Header.Get("x-ratelimit-reset-tokens")

	if reqRem != "" {
		config.RateRemainingRequests = reqRem
		config.RateRemainingTokens = tokRem
		config.RateLimitRequests = reqLim
		config.RateLimitTokens = tokLim
		config.RateResetTokens = tokRes
		saveConfig()

		if mw != nil {
			mw.Synchronize(func() {
				if lblReqRem != nil {
					lblReqRem.SetText(reqRem)
					lblTokRem.SetText(tokRem)
					lblTokRes.SetText(tokRes)
				}
			})
		}
	}

	var groqResp GroqResponse
	if err := json.NewDecoder(resp.Body).Decode(&groqResp); err != nil {
		return rawText
	}

	if len(groqResp.Choices) > 0 {
		return strings.TrimSpace(groqResp.Choices[0].Message.Content)
	}

	return rawText
}

type KEYBDINPUT struct {
	wVk         uint16
	wScan       uint16
	dwFlags     uint32
	time        uint32
	dwExtraInfo uintptr
}

type INPUT struct {
	type_   uint32
	ki      KEYBDINPUT
	padding uint64
}

const (
	INPUT_KEYBOARD  = 1
	KEYEVENTF_KEYUP = 0x0002
	VK_CONTROL      = 0x11
	VK_V            = 0x56
)

func sendCtrlV() {
	var inputs []INPUT

	inputs = append(inputs, INPUT{
		type_: INPUT_KEYBOARD,
		ki:    KEYBDINPUT{wVk: VK_CONTROL},
	})
	inputs = append(inputs, INPUT{
		type_: INPUT_KEYBOARD,
		ki:    KEYBDINPUT{wVk: VK_V},
	})
	inputs = append(inputs, INPUT{
		type_: INPUT_KEYBOARD,
		ki:    KEYBDINPUT{wVk: VK_V, dwFlags: KEYEVENTF_KEYUP},
	})
	inputs = append(inputs, INPUT{
		type_: INPUT_KEYBOARD,
		ki:    KEYBDINPUT{wVk: VK_CONTROL, dwFlags: KEYEVENTF_KEYUP},
	})

	procSendInput.Call(
		uintptr(len(inputs)),
		uintptr(unsafe.Pointer(&inputs[0])),
		uintptr(unsafe.Sizeof(INPUT{})),
	)
}

func monitorLoop() {
	lastText, _ := clipboard.ReadAll()

	for {
		time.Sleep(300 * time.Millisecond)
		currentText, err := clipboard.ReadAll()
		if err != nil || currentText == "" || currentText == lastText {
			continue
		}

		targetApp := strings.ToLower(strings.TrimSpace(config.TargetApp))
		owner := strings.ToLower(getClipboardOwnerProcess())

		if targetApp != "" && owner != "" && !strings.Contains(owner, targetApp) {
			lastText = currentText
			continue
		}

		cleanText := formatTextWithLLM(currentText)

		logTranscription(currentText, cleanText)

		lastText = cleanText
		clipboard.WriteAll(cleanText)

		time.Sleep(100 * time.Millisecond)
		sendCtrlV()
	}
}

func main() {
	mutexName, _ := syscall.UTF16PtrFromString("HandyProxy_SingleInstance_Mutex")
	handle, _, mutexErr := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(mutexName)))
	
	if handle != 0 && mutexErr != nil && mutexErr.(syscall.Errno) == ERROR_ALREADY_EXISTS {
		title, _ := syscall.UTF16PtrFromString("HandyProxy")
		hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(title)))
		if hwnd != 0 {
			procShowWindow.Call(hwnd, SW_RESTORE)
			procSetForegroundWindow.Call(hwnd)
		}
		return
	}
	if handle == 0 {
		return
	}
	defer procCloseHandle.Call(handle)

	initPaths()
	loadConfig()

	go monitorLoop()

	var apiKeyEdit, targetAppEdit *walk.LineEdit
	var modelComboBox *walk.ComboBox
	var systemPromptEdit *walk.TextEdit

	models := []string{"llama-3.1-8b-instant (Recommended)", "openai/gpt-oss-20b", "qwen/qwen3-32b"}
	modelIndex := 0
	for i, m := range models {
		if strings.Replace(m, " (Recommended)", "", 1) == config.Model {
			modelIndex = i
			break
		}
	}

	MainWindow{
		AssignTo: &mw,
		Title:    "HandyProxy",
		MinSize:  Size{Width: 800, Height: 580},
		Size:     Size{Width: 800, Height: 580},
		Font:     Font{Family: "Segoe UI", PointSize: 10},
		Layout:   VBox{},
		Children: []Widget{
			Composite{
				Layout: Grid{Columns: 2},
				Children: []Widget{
					Label{Text: "Groq API Key:"},
					LineEdit{
						AssignTo:     &apiKeyEdit,
						Text:         config.ApiKey,
						PasswordMode: true,
					},
					Label{Text: "Model Name:"},
					ComboBox{
						AssignTo:     &modelComboBox,
						Model:        models,
						CurrentIndex: modelIndex,
					},
					Label{Text: "Target App Filter:"},
					LineEdit{
						AssignTo: &targetAppEdit,
						Text:     config.TargetApp,
					},
				},
			},
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					Label{Text: "System Prompt (Keep %s for Raw Text insertion):"},
					HSpacer{},
					PushButton{
						Text: "Edit Prompt",
						OnClicked: func() {
							systemPromptEdit.SetReadOnly(false)
						},
					},
				},
			},
			TextEdit{
				AssignTo: &systemPromptEdit,
				Text:     strings.ReplaceAll(strings.ReplaceAll(config.SystemPrompt, "\r\n", "\n"), "\n", "\r\n"),
				VScroll:  true,
				ReadOnly: true,
			},
			GroupBox{
				Title:  "Groq API Usage (Live Updates)",
				Layout: HBox{},
				Children: []Widget{
					Label{Text: "Remaining Requests:"},
					Label{AssignTo: &lblReqRem, Text: config.RateRemainingRequests, ToolTipText: "Remaining API requests for today"},
					
					Label{Text: "     "}, // Spacer
					
					Label{Text: "Remaining Tokens:"},
					Label{AssignTo: &lblTokRem, Text: config.RateRemainingTokens, ToolTipText: "Remaining tokens for this minute"},
					
					Label{Text: "     "}, // Spacer
					
					Label{Text: "Reset In:"},
					Label{AssignTo: &lblTokRes, Text: config.RateResetTokens, ToolTipText: "Time until token limit resets"},
					
					HSpacer{}, // Force entire block to align left
					PushButton{
						Text: "Refresh",
						OnClicked: func() {
							go refreshGroqLimits()
						},
					},
				},
			},
			PushButton{
				Text: "Save Settings",
				OnClicked: func() {
					config.ApiKey = apiKeyEdit.Text()
					config.Model = strings.Replace(modelComboBox.Text(), " (Recommended)", "", 1)
					config.TargetApp = targetAppEdit.Text()
					config.SystemPrompt = strings.ReplaceAll(systemPromptEdit.Text(), "\r\n", "\n")
					systemPromptEdit.SetReadOnly(true)
					saveConfig()
					walk.MsgBox(mw, "Saved", "Settings saved successfully!\nActive in the background.", walk.MsgBoxIconInformation)
					mw.Hide()
				},
			},
		},
	}.Create()

	icon, _ := walk.NewIconFromResourceId(10)
	if icon == nil {
		// Fallback 1: try ID 2 or 1 which is common for rsrc
		icon, _ = walk.NewIconFromResourceId(2)
	}
	if icon == nil {
		icon, _ = walk.NewIconFromResourceId(1)
	}
	if icon == nil {
		exePath, _ := os.Executable()
		iconPath := filepath.Join(filepath.Dir(exePath), "icon-new.ico")
		icon, _ = walk.NewIconFromFile(iconPath)
	}
	if icon == nil {
		icon, _ = walk.NewIconFromFile("icon-new.ico")
	}
	if icon == nil {
		icon = walk.IconApplication()
	}
	
	if icon != nil {
		mw.SetIcon(icon)
	}

	var err error
	ni, err = walk.NewNotifyIcon(mw)
	if err != nil {
		log.Fatal(err)
	}
	defer ni.Dispose()

	if icon != nil {
		ni.SetIcon(icon)
		ni.SetVisible(true)
	}

	ni.SetToolTip("Handy Proxy")

	ni.MouseDown().Attach(func(x, y int, button walk.MouseButton) {
		if button == walk.LeftButton {
			mw.Show()
		}
	})

	showAction := walk.NewAction()
	showAction.SetText("Show Settings")
	showAction.Triggered().Attach(func() {
		mw.Show()
	})
	ni.ContextMenu().Actions().Add(showAction)

	exitAction := walk.NewAction()
	exitAction.SetText("Exit")
	exitAction.Triggered().Attach(func() {
		walk.App().Exit(0)
	})
	ni.ContextMenu().Actions().Add(exitAction)

	mw.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		*canceled = true
		mw.Hide()
	})

	mw.Run()
}

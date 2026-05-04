import { useState, useEffect, useRef, useCallback } from 'react'
import { SendMessage, GetCharacters, SetCharacter, StartPTT, StopPTT, SetVAD } from '../wailsjs/go/main/App'
import { EventsOn } from '../wailsjs/runtime/runtime'
import type { main } from '../wailsjs/go/models'
import zundamonImg from './assets/images/zundamon_normal.png'
import metanImg from './assets/images/metan_normal.png'
import tsumugiImg from './assets/images/tsumugi_normal.png'

// ── Types ────────────────────────────────────────────────────────────

interface Message {
  id: string
  role: 'user' | 'assistant' | 'thinking' | 'system'
  text: string
  emotion: string
  speaker: string
  streaming: boolean
}

type AppStatus = 'idle' | 'thinking' | 'speaking' | 'recording' | 'transcribing'

// ── Constants ────────────────────────────────────────────────────────

const CHAR_ORDER = ['zundamon', 'metan', 'tsumugi']

const CHAR_COLOR: Record<string, string> = {
  zundamon: '#4CAF50',
  metan:    '#FF6B9D',
  tsumugi:  '#FF8C00',
}

const CHAR_INITIAL: Record<string, string> = {
  zundamon: 'ず',
  metan:    'め',
  tsumugi:  'つ',
}

const CHAR_IMAGE: Record<string, string> = {
  zundamon: zundamonImg,
  metan:    metanImg,
  tsumugi:  tsumugiImg,
}

const CHAR_TAGLINE: Record<string, string> = {
  zundamon: 'ずんだ餅の精霊',
  metan:    'ポジティブ系',
  tsumugi:  '埼玉出身 JK',
}

const STATUS_LABEL: Record<AppStatus, string> = {
  idle:         '待機中',
  thinking:     '考え中...',
  speaking:     '話し中',
  recording:    '録音中...',
  transcribing: '文字起こし中...',
}

const THINKING_ID = '__thinking__'

// ── Helpers ──────────────────────────────────────────────────────────

function lastAssistantId(messages: Message[]): string | null {
  for (let i = messages.length - 1; i >= 0; i--) {
    if (messages[i].role === 'assistant') return messages[i].id
  }
  return null
}

// ── Component ────────────────────────────────────────────────────────

function App() {
  const [messages, setMessages]           = useState<Message[]>([])
  const [input, setInput]                 = useState('')
  const [status, setStatus]               = useState<AppStatus>('idle')
  const [characters, setCharacters]       = useState<main.CharacterInfo[]>([])
  const [activeCharKey, setActiveCharKey] = useState('zundamon')
  const [speakingMsgId, setSpeakingMsgId] = useState<string | null>(null)
  const [error, setError]                 = useState<string | null>(null)
  const [charSwitching, setCharSwitching] = useState(false)
  const [isPTTRecording, setIsPTTRecording] = useState(false)
  const [isVADOn, setIsVADOn]             = useState(false)

  const messagesEndRef = useRef<HTMLDivElement>(null)
  const inputRef       = useRef<HTMLTextAreaElement>(null)
  const messagesRef    = useRef<Message[]>(messages)
  // ptt ボタンを離したときに確実に StopPTT を呼ぶための ref
  const isPTTRef       = useRef(false)

  useEffect(() => { messagesRef.current = messages }, [messages])
  useEffect(() => { isPTTRef.current = isPTTRecording }, [isPTTRecording])

  // ── Wails event listeners ──
  useEffect(() => {
    GetCharacters().then((chars) => {
      const sorted = [...chars].sort(
        (a, b) => CHAR_ORDER.indexOf(a.key) - CHAR_ORDER.indexOf(b.key),
      )
      setCharacters(sorted)
    })

    const offSentence = EventsOn(
      'message:sentence',
      (data: { text: string; emotion: string; speaker: string }) => {
        setMessages((prev) => {
          const withoutThinking = prev.filter((m) => m.id !== THINKING_ID)
          const last = withoutThinking[withoutThinking.length - 1]
          if (last?.role === 'assistant' && last.streaming) {
            return [
              ...withoutThinking.slice(0, -1),
              { ...last, text: last.text + data.text, emotion: data.emotion, speaker: data.speaker },
            ]
          }
          return [
            ...withoutThinking,
            {
              id: Date.now().toString(),
              role: 'assistant' as const,
              text: data.text,
              emotion: data.emotion,
              speaker: data.speaker,
              streaming: true,
            },
          ]
        })
      },
    )

    const offStatus = EventsOn(
      'status:changed',
      (data: { status: AppStatus }) => {
        setStatus(data.status)

        if (data.status === 'thinking') {
          setSpeakingMsgId(null)
          setMessages((prev) => {
            if (prev.some((m) => m.id === THINKING_ID)) return prev
            return [
              ...prev,
              { id: THINKING_ID, role: 'thinking' as const, text: '', emotion: '', speaker: '', streaming: false },
            ]
          })
        }

        if (data.status === 'speaking') {
          setSpeakingMsgId(lastAssistantId(messagesRef.current))
        }

        if (data.status === 'idle') {
          setSpeakingMsgId(null)
          setIsPTTRecording(false)
        }
      },
    )

    const offDone = EventsOn('message:done', () => {
      setStatus('idle')
      setSpeakingMsgId(null)
      setMessages((prev) =>
        prev
          .filter((m) => m.id !== THINKING_ID)
          .map((m, i, arr) =>
            i === arr.length - 1 && m.role === 'assistant'
              ? { ...m, streaming: false }
              : m,
          ),
      )
      inputRef.current?.focus()
    })

    // 音声認識結果をユーザーメッセージとしてチャットに追加
    const offTranscribed = EventsOn('recording:transcribed', (data: { text: string }) => {
      if (!data.text) return
      setMessages((prev) => [
        ...prev,
        {
          id: Date.now().toString(),
          role: 'user' as const,
          text: data.text,
          emotion: '',
          speaker: '',
          streaming: false,
        },
      ])
    })

    const offError = EventsOn('error:occurred', (data: { message: string }) => {
      setError(data.message)
      setStatus('idle')
      setSpeakingMsgId(null)
      setIsPTTRecording(false)
      setMessages((prev) => prev.filter((m) => m.id !== THINKING_ID))
    })

    return () => {
      offSentence()
      offStatus()
      offDone()
      offTranscribed()
      offError()
    }
  }, [])

  // Auto-scroll
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  // ── テキスト送信 ──
  const handleSend = useCallback(async () => {
    const text = input.trim()
    if (!text || status !== 'idle') return
    setInput('')
    setError(null)
    setMessages((prev) => [
      ...prev,
      { id: Date.now().toString(), role: 'user', text, emotion: '', speaker: '', streaming: false },
    ])
    setStatus('thinking')
    try {
      await SendMessage(text)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
      setStatus('idle')
      setSpeakingMsgId(null)
      setMessages((prev) => prev.filter((m) => m.id !== THINKING_ID))
    }
  }, [input, status])

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && e.metaKey) {
      e.preventDefault()
      handleSend()
    }
  }

  // ── PTT ──
  const handlePTTStart = async () => {
    if (isVADOn || status !== 'idle') return
    try {
      await StartPTT()
      setIsPTTRecording(true)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  const handlePTTStop = async () => {
    if (!isPTTRef.current) return
    setIsPTTRecording(false)
    try {
      await StopPTT()
      // status will change to transcribing → thinking → speaking → idle via events
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
      setStatus('idle')
    }
  }

  // ── VAD ──
  const handleVADToggle = async () => {
    if (isPTTRecording) return
    const next = !isVADOn
    try {
      await SetVAD(next)
      setIsVADOn(next)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  // ── キャラクター切替 ──
  const handleCharChange = async (key: string) => {
    if (key === activeCharKey || status !== 'idle' || charSwitching) return
    setCharSwitching(true)
    try {
      await SetCharacter(key)
      const prev = activeCharKey
      setActiveCharKey(key)
      setError(null)
      setMessages((msgs) => {
        if (msgs.length === 0) return msgs
        const char     = characters.find((c) => c.key === key)
        const prevChar = characters.find((c) => c.key === prev)
        const label = prevChar
          ? `${prevChar.name} → ${char?.name ?? key}`
          : `${char?.name ?? key} に変更`
        return [
          ...msgs,
          { id: Date.now().toString(), role: 'system' as const, text: label, emotion: '', speaker: '', streaming: false },
        ]
      })
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setCharSwitching(false)
    }
  }

  const activeColor = CHAR_COLOR[activeCharKey] ?? '#4CAF50'
  const isBusy = status !== 'idle'
  // VAD 中でも録音中・文字起こし中・thinking 以外は "見かけ上 idle" なので入力欄操作を許可しない
  const isInputDisabled = isBusy || isVADOn

  return (
    <div className="app" style={{ '--active-color': activeColor } as React.CSSProperties}>

      {/* ── Header ── */}
      <header className="header" style={{ borderBottomColor: activeColor }}>
        <div className="char-tabs">
          {characters.map((c) => {
            const isActive = activeCharKey === c.key
            const color = CHAR_COLOR[c.key] ?? '#4CAF50'
            console.log(c.key, CHAR_IMAGE[c.key])
            return (
              <button
                key={c.key}
                className={'char-tab' + (isActive ? ' active' : '') + (charSwitching ? ' switching' : '')}
                style={isActive ? { borderColor: color } : {}}
                onClick={() => handleCharChange(c.key)}
                disabled={isBusy || charSwitching}
                title={isActive ? undefined : c.name}
              >
                <span
                  className="char-avatar"
                  style={{ background: isActive ? color : 'transparent', borderColor: color }}
                >
                  {CHAR_IMAGE[c.key]
                    ? <img src={`${CHAR_IMAGE[c.key]}?v=1`}  alt="" className={isActive ? '' : 'dimmed'} />
                    : (CHAR_INITIAL[c.key] ?? c.name[0])
                  }
                </span>
                <span className="char-label">
                  <span className="char-name" style={isActive ? { color } : {}}>{c.name}</span>
                  {isActive && (
                    <span className="char-tagline">{CHAR_TAGLINE[c.key]}</span>
                  )}
                </span>
              </button>
            )
          })}
        </div>

        <div
          className={'status-badge' + (isBusy ? ' busy' : '')}
          style={isBusy ? { color: activeColor } : undefined}
        >
          {status === 'speaking'   && <SoundWave color={activeColor} />}
          {status === 'recording'  && <RecordingPulse />}
          {STATUS_LABEL[status]}
        </div>
      </header>

      {/* ── Error banner ── */}
      {error && (
        <div className="error-banner">
          <span>⚠ {error}</span>
          <button onClick={() => setError(null)}>✕</button>
        </div>
      )}

      {/* ── Chat window ── */}
      <main className="chat-window">
        {messages.length === 0 && (
          <div className="empty-state">
            <CharAvatar charKey={activeCharKey} size={56} />
            <p className="empty-char-name" style={{ color: activeColor }}>
              {characters.find((c) => c.key === activeCharKey)?.name ?? ''}
            </p>
            <p className="empty-tagline">{CHAR_TAGLINE[activeCharKey]}</p>
            <p className="empty-hint">テキストを入力、またはマイクボタンで話しかけてみよう</p>
          </div>
        )}

        {messages.map((msg) => {
          if (msg.role === 'thinking') {
            return (
              <div key={msg.id} className="bubble-row assistant">
                <div className="bubble assistant thinking-bubble" style={{ borderColor: activeColor }}>
                  <div className="meta" style={{ color: activeColor }}><span>💭</span></div>
                  <div className="thinking-dots"><span /><span /><span /></div>
                </div>
              </div>
            )
          }

          if (msg.role === 'system') {
            return (
              <div key={msg.id} className="system-message">
                <span>{msg.text}</span>
              </div>
            )
          }

          const isSpeakingThis = msg.id === speakingMsgId
          const msgCharKey = characters.find(c => c.name === msg.speaker || c.key === msg.speaker)?.key ?? activeCharKey
          return (
            <div key={msg.id} className={'bubble-row ' + msg.role}>
              <div
                className={['bubble', msg.role, isSpeakingThis ? 'is-speaking' : ''].filter(Boolean).join(' ')}
                style={
                  msg.role === 'assistant'
                    ? { borderColor: activeColor, ...(isSpeakingThis ? { '--glow-color': activeColor } as React.CSSProperties : {}) }
                    : undefined
                }
              >
                {msg.role === 'assistant' && (
                  <div className="meta" style={{ color: activeColor }}>
                    <span className="msg-avatar" style={{ borderColor: CHAR_COLOR[msgCharKey] }}>
                      {CHAR_IMAGE[msgCharKey] && <img src={CHAR_IMAGE[msgCharKey]} alt="" />}
                    </span>
                    <span className="speaker-name">{msg.speaker}</span>
                    {isSpeakingThis && <SoundWave color={activeColor} small />}
                  </div>
                )}
                <p>
                  {msg.text}
                  {msg.streaming && <span className="cursor">▍</span>}
                </p>
              </div>
            </div>
          )
        })}

        <div ref={messagesEndRef} />
      </main>

      {/* ── Input bar ── */}
      <footer className="input-bar">
        {/* PTT ボタン */}
        <button
          className={`voice-btn ptt-btn${isPTTRecording ? ' recording' : ''}`}
          onMouseDown={handlePTTStart}
          onMouseUp={handlePTTStop}
          onMouseLeave={handlePTTStop}
          disabled={isVADOn || (isBusy && !isPTTRecording)}
          title="押している間だけ録音 (PTT)"
        >
          {isPTTRecording
            ? <><RecordingPulse /><span>REC</span></>
            : <><MicIcon /><span>PTT</span></>
          }
        </button>

        {/* VAD トグルボタン */}
        <button
          className={`voice-btn vad-btn${isVADOn ? ' active' : ''}`}
          style={isVADOn ? { borderColor: activeColor, color: activeColor } : {}}
          onClick={handleVADToggle}
          disabled={isPTTRecording || (isBusy && !isVADOn)}
          title={isVADOn ? 'VAD をオフにする' : '音声自動検知をオン (VAD)'}
        >
          <MicIcon />
          <span>{isVADOn ? 'VAD ON' : 'VAD'}</span>
          {isVADOn && <span className="vad-dot" />}
        </button>

        <textarea
          ref={inputRef}
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={
            isVADOn
              ? '🔴 VAD 録音中... (VAD ボタンでオフ)'
              : 'メッセージを入力... (⌘Enter で送信 / Enter で改行)'
          }
          disabled={isInputDisabled}
          rows={2}
        />

        <button
          onClick={handleSend}
          disabled={isInputDisabled || !input.trim()}
          className="send-btn"
          style={{ background: activeColor }}
        >
          送信
        </button>
      </footer>
    </div>
  )
}

// ── Sub-components ───────────────────────────────────────────────────

function CharAvatar({ charKey, size = 26 }: { charKey: string; size?: number }) {
  const img = CHAR_IMAGE[charKey]
  return (
    <span
      className="char-avatar standalone"
      style={{
        background: CHAR_COLOR[charKey] ?? '#4CAF50',
        width: size,
        height: size,
        fontSize: size * 0.42,
        borderColor: CHAR_COLOR[charKey] ?? '#4CAF50',
      }}
    >
      {img
        ? <img src={img} alt={charKey} />
        : (CHAR_INITIAL[charKey] ?? '?')
      }
    </span>
  )
}

function SoundWave({ color, small = false }: { color: string; small?: boolean }) {
  return (
    <span className={'sound-wave' + (small ? ' small' : '')} aria-label="再生中">
      {[1, 2, 3, 4].map((i) => (
        <span key={i} style={{ background: color }} />
      ))}
    </span>
  )
}

function RecordingPulse() {
  return <span className="recording-pulse" aria-label="録音中" />
}

function MicIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor">
      <path d="M12 14a3 3 0 003-3V5a3 3 0 00-6 0v6a3 3 0 003 3zm5-3a5 5 0 01-10 0H5a7 7 0 0014 0h-2zm-5 9v-2.07A7.003 7.003 0 005.07 11H3a9 9 0 0018 0h-2.07A7.003 7.003 0 0012 20v2H9v2h6v-2h-3z"/>
    </svg>
  )
}

export default App

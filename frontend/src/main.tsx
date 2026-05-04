import ReactDOM from 'react-dom/client'
import App from './App'
import './App.css'

// React.StrictMode causes useEffect to fire twice in dev, which double-registers
// Wails event listeners and results in duplicate text display.
ReactDOM.createRoot(document.getElementById('root')!).render(<App />)

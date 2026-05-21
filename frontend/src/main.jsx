import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.jsx'

// StrictMode removed: воно викликає подвійні запити до API у dev-режимі,
// оскільки навмисно двічі запускає useEffect для пошуку побічних ефектів.
createRoot(document.getElementById('root')).render(
  <App />
)

import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom';
import Login from './pages/Login';
import Projects from './pages/Projects';

// Компонент-обгортка для захищених маршрутів
function ProtectedRoute({ children }) {
    const token = localStorage.getItem('token');
    if (!token) {
        return <Navigate to="/login" replace />;
    }
    return children;
}

export default function App() {
    return (
        <Router>
            <Routes>
                {/* Відкритий маршрут */}
                <Route path="/login" element={<Login />} />

                {/* Захищені маршрути */}
                <Route 
                    path="/" 
                    element={
                        <ProtectedRoute>
                            <Projects />
                        </ProtectedRoute>
                    } 
                />
                
                {/* Якщо сторінки не існує — кидаємо на головну */}
                <Route path="*" element={<Navigate to="/" replace />} />
            </Routes>
        </Router>
    );
}
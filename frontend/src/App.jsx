import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom';
import Login from './pages/Login';
import Projects from './pages/Projects';
import ProjectDetail from './pages/ProjectDetail';
import CategoryDetail from './pages/CategoryDetail';

function ProtectedRoute({ children }) {
    const token = localStorage.getItem('token');
    if (!token) return <Navigate to="/login" replace />;
    return children;
}

export default function App() {
    return (
        <Router>
            <Routes>
                <Route path="/login" element={<Login />} />
                <Route path="/" element={<ProtectedRoute><Projects /></ProtectedRoute>} />
                <Route path="/projects/:pid" element={<ProtectedRoute><ProjectDetail /></ProtectedRoute>} />
                <Route path="/projects/:pid/categories/:cid" element={<ProtectedRoute><CategoryDetail /></ProtectedRoute>} />
                <Route path="*" element={<Navigate to="/" replace />} />
            </Routes>
        </Router>
    );
}

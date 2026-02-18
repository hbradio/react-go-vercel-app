import { Routes, Route, Link } from 'react-router-dom'
import { useAuth0 } from '@auth0/auth0-react'
import AuthButtons from './components/AuthButtons'
import ProtectedRoute from './components/ProtectedRoute'
import Home from './pages/Home'
import Dashboard from './pages/Dashboard'
import Premium from './pages/Premium'
import './App.css'

function App() {
  const { isLoading } = useAuth0()

  if (isLoading) {
    return <div className="loading">Loading...</div>
  }

  return (
    <div className="app">
      <nav>
        <Link to="/">Home</Link>
        <Link to="/dashboard">Dashboard</Link>
        <Link to="/premium">Premium</Link>
        <AuthButtons />
      </nav>
      <main>
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/dashboard" element={<ProtectedRoute><Dashboard /></ProtectedRoute>} />
          <Route path="/premium" element={<ProtectedRoute><Premium /></ProtectedRoute>} />
        </Routes>
      </main>
    </div>
  )
}

export default App

import { useAuth0 } from '@auth0/auth0-react'
import { useEffect, useState } from 'react'
import { useApi } from '../lib/api'
import PurchaseGate from '../components/PurchaseGate'

interface UserData {
  id: string
  email: string
  has_purchased: boolean
  created_at: string
}

export default function Dashboard() {
  const { user } = useAuth0()
  const { fetchWithAuth } = useApi()
  const [userData, setUserData] = useState<UserData | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    fetchWithAuth('/api/user')
      .then(setUserData)
      .catch((err: Error) => setError(err.message))
  }, [fetchWithAuth])

  return (
    <div className="dashboard">
      <h1>Dashboard</h1>

      {error && <p style={{ color: 'red' }}>Error: {error}</p>}

      <div className="user-info">
        <h2>Profile</h2>
        <p><strong>Name:</strong> {user?.name}</p>
        <p><strong>Email:</strong> {user?.email}</p>
        {userData && (
          <>
            <p><strong>Status:</strong> {userData.has_purchased ? 'Premium' : 'Free'}</p>
            <p><strong>Member since:</strong> {new Date(userData.created_at).toLocaleDateString()}</p>
          </>
        )}
      </div>

      {userData && !userData.has_purchased && (
        <PurchaseGate hasPurchased={false}>
          <span />
        </PurchaseGate>
      )}

      {userData?.has_purchased && (
        <p style={{ color: '#4caf50', marginTop: '1rem' }}>
          You have premium access. <a href="/premium">View premium content</a>
        </p>
      )}
    </div>
  )
}

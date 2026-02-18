import { useEffect, useState } from 'react'
import { useApi } from '../lib/api'
import PurchaseGate from '../components/PurchaseGate'

interface UserData {
  has_purchased: boolean
}

export default function Premium() {
  const { fetchWithAuth } = useApi()
  const [userData, setUserData] = useState<UserData | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetchWithAuth('/api/user')
      .then((data: UserData) => {
        setUserData(data)
        setLoading(false)
      })
      .catch(() => setLoading(false))
  }, [fetchWithAuth])

  if (loading) {
    return <div>Loading...</div>
  }

  return (
    <div>
      <h1>Premium Content</h1>
      <PurchaseGate hasPurchased={userData?.has_purchased ?? false}>
        <div className="premium-content">
          <p>This content is only visible to users who have purchased access.</p>
          <p>You now have full access to all premium features.</p>
        </div>
      </PurchaseGate>
    </div>
  )
}

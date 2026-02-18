import { ReactNode, useState } from 'react'
import { useApi } from '../lib/api'

interface Props {
  hasPurchased: boolean
  children: ReactNode
}

export default function PurchaseGate({ hasPurchased, children }: Props) {
  const { fetchWithAuth } = useApi()
  const [loading, setLoading] = useState(false)

  if (hasPurchased) {
    return <>{children}</>
  }

  const handlePurchase = async () => {
    setLoading(true)
    try {
      const data = await fetchWithAuth('/api/create-checkout', { method: 'POST' })
      window.location.href = data.url
    } catch (err) {
      console.error('Checkout error:', err)
      setLoading(false)
    }
  }

  return (
    <div className="purchase-gate">
      <h3>Premium Content</h3>
      <p>Purchase access to unlock this content.</p>
      <button onClick={handlePurchase} disabled={loading}>
        {loading ? 'Redirecting...' : 'Buy Access — $9.99'}
      </button>
    </div>
  )
}

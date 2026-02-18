import { useAuth0 } from '@auth0/auth0-react'
import type { ReactNode } from 'react'

export default function ProtectedRoute({ children }: { children: ReactNode }) {
  const { isAuthenticated, isLoading, loginWithRedirect } = useAuth0()

  if (isLoading) {
    return <div>Loading...</div>
  }

  if (!isAuthenticated) {
    loginWithRedirect()
    return <div>Redirecting to login...</div>
  }

  return <>{children}</>
}

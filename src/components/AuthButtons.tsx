import { useAuth0 } from '@auth0/auth0-react'

export default function AuthButtons() {
  const { isAuthenticated, loginWithRedirect, logout, user } = useAuth0()

  if (isAuthenticated) {
    return (
      <span style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
        <span>{user?.email}</span>
        <button onClick={() => logout({ logoutParams: { returnTo: window.location.origin } })}>
          Log out
        </button>
      </span>
    )
  }

  return (
    <button style={{ marginLeft: 'auto' }} onClick={() => loginWithRedirect()}>
      Log in
    </button>
  )
}

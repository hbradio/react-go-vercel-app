import { useAuth0 } from '@auth0/auth0-react'
import { Link } from 'react-router-dom'

export default function Home() {
  const { isAuthenticated, loginWithRedirect } = useAuth0()

  return (
    <div>
      <h1>React + Go Starter</h1>
      <p>A serverless starter app with Auth0, Stripe, and CockroachDB.</p>
      {isAuthenticated ? (
        <p><Link to="/dashboard">Go to Dashboard</Link></p>
      ) : (
        <p>
          <button onClick={() => loginWithRedirect()}>Log in to get started</button>
        </p>
      )}
    </div>
  )
}

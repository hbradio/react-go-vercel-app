import { useAuth0 } from '@auth0/auth0-react'
import { useCallback } from 'react'

export function useApi() {
  const { getAccessTokenSilently } = useAuth0()

  const fetchWithAuth = useCallback(
    async (url: string, options: RequestInit = {}) => {
      const token = await getAccessTokenSilently()
      const res = await fetch(url, {
        ...options,
        headers: {
          ...options.headers,
          Authorization: `Bearer ${token}`,
          'Content-Type': 'application/json',
        },
      })
      if (!res.ok) {
        throw new Error(`API error: ${res.status}`)
      }
      return res.json()
    },
    [getAccessTokenSilently]
  )

  return { fetchWithAuth }
}

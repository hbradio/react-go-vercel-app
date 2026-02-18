import { execSync } from 'child_process'
import { readFileSync } from 'fs'

// Read .env file and extract STRIPE_SECRET_KEY
const env = readFileSync('.env', 'utf8')
const match = env.match(/^STRIPE_SECRET_KEY=(.+)$/m)
if (!match) {
  console.error('STRIPE_SECRET_KEY not found in .env')
  process.exit(1)
}
const key = match[1].trim()

try {
  execSync(
    `docker run --rm --name stripe-cli stripe/stripe-cli listen --api-key ${key} --forward-to http://host.docker.internal:8089/api/webhook -s`,
    { stdio: 'inherit' }
  )
} catch {
  process.exit(1)
}

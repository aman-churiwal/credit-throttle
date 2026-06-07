import http from 'k6/http'
import { check, sleep } from 'k6'

export const options = {
    vus: 50,
    duration: '10s'
}

const BASE_URL = 'http://localhost:8080'
const ACCOUNT_ID = '0c1d60d7-811b-49e2-93a2-e0a38902a9eb'
const SPEND_AMOUNT = 10

export default function () {
    // All VUs use the same fixed key
    const idempotencyKey = 'fixed-idempotency-key-test'

    const params = {
        headers: {
            'Content-Type': 'application/json',
            'X-Idempotency-Key': idempotencyKey
        },
    }

    const payload = JSON.stringify({
        account_id: ACCOUNT_ID,
        amount: SPEND_AMOUNT,
    })

    const res = http.post(`${BASE_URL}/spend`, payload, params)

    check(res, {
        'status is 201 or 422': (r) => r.status === 201 || r.status === 422,
        'no 500 errors': (r) => r.status !== 500,
        'no 429 errors': (r) => r.status !== 429,
    })
    sleep(0.1)
}

const { defineConfig } = require('cypress')
const { execFileSync } = require('node:child_process')
const { Client } = require('pg')

const db = process.env.E2E_DATABASE_URL || 'postgres://radiopath:radiopath@localhost:5432/radiopath_e2e'

module.exports = defineConfig({
  e2e: {
    baseUrl: process.env.E2E_BASE_URL || 'http://localhost:8081',
    expose: { mailpit: process.env.E2E_MAILPIT_URL || 'http://localhost:8025' },
    env: { adminToken: process.env.E2E_ADMIN_TOKEN },
    allowCypressEnv: false,
    video: false,
    setupNodeEvents(on) {
      on('task', {
        async reset() {
          const c = new Client({ connectionString: db })
          await c.connect()
          await c.query('TRUNCATE users, rate_limits, settings CASCADE')
          await c.end()
          return null
        },
        useradd({ name, password }) {
          execFileSync(process.env.E2E_BIN || './radiopath', ['useradd', name], {
            input: password + '\n',
            env: { ...process.env, DATABASE_URL: db },
          })
          return null
        },
      })
    },
  },
})

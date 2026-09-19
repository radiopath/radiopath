beforeEach(() => {
  cy.intercept('/tiles/**', { statusCode: 204 })
})

const mailpit = () => Cypress.expose('mailpit')

Cypress.Commands.add('wipe', () => {
  cy.task('reset')
  cy.request('DELETE', `${mailpit()}/api/v1/messages`)
})

Cypress.Commands.add('useradd', (name, password) => cy.task('useradd', { name, password }))

// login without the form: a request carries no Origin, so the CSRF check lets it pass
Cypress.Commands.add('login', (name, password) => {
  cy.request({ method: 'POST', url: '/login', form: true, body: { name, password, next: '/' } })
})

Cypress.Commands.add('mail', (to) => {
  cy.request(`${mailpit()}/api/v1/search?query=${encodeURIComponent('to:' + to)}`)
    .its('body.messages').should('have.length.at.least', 1)
    .then((msgs) => cy.request(`${mailpit()}/api/v1/message/${msgs[0].ID}`))
    .its('body.Text')
})

Cypress.Commands.add('mailLink', (to) => {
  cy.mail(to).then((text) => text.match(/https?:\/\/\S+/)[0])
})

Cypress.Commands.add('addSite', (name, lat, lon, height = 10) => {
  cy.request({ method: 'POST', url: '/sites', form: true, body: { name, lat, lon, antenna_height_m: height } })
  cy.request('/sites').its('body').then((html) => {
    const m = html.match(new RegExp(`/sites/(\\d+)">${name}<`))
    expect(m, `site ${name} listed`).to.not.be.null
    return Number(m[1])
  })
})

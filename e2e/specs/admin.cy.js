describe('admin panel', () => {
  before(() => {
    cy.wipe()
    cy.useradd('alice', 'alice-password-1')
  })

  it('needs the token', () => {
    cy.visit('/admin')
    cy.contains('h1', 'Admin')
    cy.get('[name=token]').type('wrong-token-wrong-token-wrong-token{enter}')
    cy.contains('.alert-error', 'Wrong token.')
    cy.request({ url: '/admin/users/alice/password', failOnStatusCode: false }).its('status').should('not.eq', 200)
  })

  it('creates a user, blocks logins, deletes the user', () => {
    cy.visit('/admin')
    cy.env(['adminToken']).then(({ adminToken }) => cy.get('[name=token]').type(adminToken + '{enter}'))
    cy.contains('h1', 'Users')
    cy.contains('tr', 'alice')

    cy.get('form[action="/admin/users"]').within(() => {
      cy.get('[name=name]').type('carol')
      cy.get('[name=password]').clear().type('carol-password-1')
      cy.contains('button', 'Create').click()
    })
    cy.contains('tr', 'carol')
    cy.contains('.table-note', '2 users')

    cy.contains('button', 'Maintenance mode').click()
    cy.contains('.badge', 'maintenance')
    cy.request({ method: 'POST', url: '/login', form: true, body: { name: 'carol', password: 'carol-password-1', next: '/' }, failOnStatusCode: false })
      .its('status').should('eq', 503)
    cy.contains('button', 'Allow logins again').click()
    cy.contains('.badge', 'open')
    cy.login('carol', 'carol-password-1')

    cy.visit('/admin')
    cy.contains('tr', 'carol').contains('button', 'Delete').click()
    cy.contains('.table-note', '1 user')
    cy.contains('tr', 'carol').should('not.exist')
  })
})

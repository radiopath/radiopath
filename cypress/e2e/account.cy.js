describe('account', () => {
  before(() => {
    cy.wipe()
    cy.useradd('alice', 'alice-password-1')
  })

  it('changes the password and logs the other device out', () => {
    cy.login('alice', 'alice-password-1')
    cy.getCookie('radiopath_session').its('value').as('other')
    cy.login('alice', 'alice-password-1')
    cy.visit('/account')
    cy.get('[name=current]').type('alice-password-1')
    cy.get('[name=password]').type('alice-password-2')
    cy.get('[name=password2]').type('alice-password-2{enter}')
    cy.contains('.alert-info', 'Password changed. Other devices have been logged out.')
    cy.get('header').contains('a', 'alice')
    cy.get('@other').then((other) => {
      cy.request({ url: '/account', headers: { Cookie: `radiopath_session=${other}` }, followRedirect: false })
        .its('status').should('eq', 303)
    })
    cy.request({ method: 'POST', url: '/login', form: true, body: { name: 'alice', password: 'alice-password-1', next: '/' }, failOnStatusCode: false })
      .its('status').should('eq', 401)
  })

  it('rejects a wrong current password', () => {
    cy.login('alice', 'alice-password-2')
    cy.visit('/account')
    cy.get('[name=current]').type('alice-password-1')
    cy.get('[name=password]').type('alice-password-3')
    cy.get('[name=password2]').type('alice-password-3{enter}')
    cy.contains('.alert-error li', 'Current password is wrong')
  })

  it('adds an address by confirmation link', () => {
    cy.login('alice', 'alice-password-2')
    cy.visit('/account')
    cy.contains('No address on file')
    cy.get('[name=email]').type('alice@example.test{enter}')
    cy.contains('.alert-info', 'Confirmation link sent to the new address')
    cy.mailLink('alice@example.test').then((link) => cy.visit(link))
    cy.location('pathname').should('eq', '/account')
    cy.contains('.alert-info', 'Address confirmed.')
    cy.get('[name=email]').should('have.value', 'alice@example.test')
  })
})

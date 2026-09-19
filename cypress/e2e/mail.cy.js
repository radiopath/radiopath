describe('mail', () => {
  before(() => cy.wipe())

  it('registers, confirms the address and logs in', () => {
    cy.visit('/login')
    cy.contains('a', 'Create an account').click()
    cy.get('[name=name]').type('dave')
    cy.get('[name=email]').type('dave@example.test')
    cy.get('[name=password]').type('dave-password-1')
    cy.get('[name=password2]').type('dave-password-1{enter}')
    cy.location('pathname').should('eq', '/login')
    cy.contains('.alert-info', 'Almost done')

    cy.get('[name=name]').type('dave')
    cy.get('[name=password]').type('dave-password-1{enter}')
    cy.contains('.alert-info', 'not confirmed yet')
    cy.getCookie('radiopath_session').should('be.null')
    cy.contains('button', 'Resend confirmation mail').click()
    cy.contains('.alert-info', 'a new link is on its way')

    cy.mailLink('dave@example.test').then((link) => cy.visit(link))
    cy.location('pathname').should('eq', '/login')
    cy.contains('.alert-info', 'Address confirmed.')
    cy.get('[name=name]').type('dave')
    cy.get('[name=password]').type('dave-password-1{enter}')
    cy.location('pathname').should('eq', '/sites')

    cy.mailLink('dave@example.test').then((link) => cy.visit({ url: link, failOnStatusCode: false }))
    cy.contains('.alert-error', 'expired or was already used')
  })

  it('resets a forgotten password', () => {
    cy.visit('/login')
    cy.contains('a', 'Forgot password?').click()
    cy.get('[name=email]').type('dave@example.test{enter}')
    cy.contains('.alert-info', 'a reset link is on its way')
    cy.mailLink('dave@example.test').then((link) => cy.visit(link))
    cy.contains('h1', 'Choose a new password')
    cy.get('[name=password]').type('dave-password-2')
    cy.get('[name=password2]').type('dave-password-2{enter}')
    cy.location('pathname').should('eq', '/login')
    cy.contains('.alert-info', 'Password changed. Log in with the new one.')
    cy.login('dave', 'dave-password-2')

    cy.mailLink('dave@example.test').then((link) => cy.visit({ url: link, failOnStatusCode: false }))
    cy.contains('.alert-error', 'expired or was already used')
    cy.get('[name=password]').should('not.exist')
  })

  it('answers the same for an unknown address', () => {
    cy.visit('/forgot')
    cy.get('[name=email]').type('nobody@example.test{enter}')
    cy.contains('.alert-info', 'a reset link is on its way')
    cy.request(`${Cypress.expose('mailpit')}/api/v1/search?query=${encodeURIComponent('to:nobody@example.test')}`)
      .its('body.messages').should('have.length', 0)
  })
})

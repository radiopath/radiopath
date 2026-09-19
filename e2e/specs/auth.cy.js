describe('login', () => {
  before(() => {
    cy.wipe()
    cy.useradd('alice', 'alice-password-1')
  })

  it('sends a guest to the login page and back', () => {
    cy.visit('/sites')
    cy.location('pathname').should('eq', '/login')
    cy.location('search').should('eq', '?next=%2Fsites')
    cy.get('[name=name]').type('alice')
    cy.get('[name=password]').type('alice-password-1{enter}')
    cy.location('pathname').should('eq', '/sites')
    cy.get('header').contains('a', 'alice')
  })

  it('rejects a wrong password', () => {
    cy.visit('/login')
    cy.get('[name=name]').type('alice')
    cy.get('[name=password]').type('not-the-password{enter}')
    cy.contains('.alert-error', 'Wrong user name or password.')
    cy.getCookie('radiopath_session').should('be.null')
  })

  it('logs out', () => {
    cy.login('alice', 'alice-password-1')
    cy.visit('/links')
    cy.contains('button', 'Logout').click()
    cy.location('pathname').should('eq', '/login')
    cy.visit('/links')
    cy.location('pathname').should('eq', '/login')
  })
})

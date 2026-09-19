describe('sites', () => {
  before(() => {
    cy.wipe()
    cy.useradd('alice', 'alice-password-1')
  })
  beforeEach(() => cy.login('alice', 'alice-password-1'))

  it('creates, edits, duplicates and deletes a site', () => {
    cy.visit('/sites')
    cy.contains('No sites yet')
    cy.contains('a', 'New site').click()
    cy.get('[name=name]').type('Säntis')
    cy.get('[name=lat]').type('47.2495')
    cy.get('[name=lon]').type('9.3433')
    cy.get('[name=antenna_height_m]').clear().type('12')
    cy.contains('button', 'Save').click()
    cy.location('pathname').should('eq', '/sites')
    cy.contains('tr', 'Säntis').within(() => {
      cy.contains('47.24950')
      cy.contains('12.0 m')
    })

    cy.contains('tr', 'Säntis').contains('a', 'Edit').click()
    cy.get('[name=antenna_height_m]').clear().type('20')
    cy.contains('button', 'Save').click()
    cy.contains('tr', 'Säntis').contains('20.0 m')

    cy.contains('tr', 'Säntis').contains('a', 'Duplicate').click()
    cy.get('[name=name]').should('have.value', 'Säntis (copy)')
    cy.get('[name=lat]').should('have.value', '47.2495')
    cy.contains('button', 'Save').click()
    cy.contains('.table-note', '2 sites')

    cy.contains('tr', 'Säntis (copy)').contains('button', 'Delete').click()
    cy.contains('.table-note', '1 site')
    cy.contains('Säntis (copy)').should('not.exist')
  })

  it('re-renders the form with the errors', () => {
    cy.visit('/sites/new')
    cy.get('[name=name]').type('Bad')
    cy.get('[name=lat]').type('95')
    cy.get('[name=lon]').type('abc')
    cy.contains('button', 'Save').click()
    cy.contains('.alert-error li', 'Latitude: must be between -90 and 90')
    cy.contains('.alert-error li', 'Longitude: not a number')
    cy.get('[name=name]').should('have.value', 'Bad')
  })
})

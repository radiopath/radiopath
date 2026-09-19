describe('ownership', () => {
  let siteID

  before(() => {
    cy.wipe()
    cy.useradd('alice', 'alice-password-1')
    cy.useradd('bob', 'bob-password-1')
    cy.login('alice', 'alice-password-1')
    cy.addSite('Alice site', 47.3, 9.3).then((id) => { siteID = id })
  })

  it("hides one user's data from another", () => {
    cy.login('bob', 'bob-password-1')
    cy.visit('/sites')
    cy.contains('No sites yet')
    cy.contains('Alice site').should('not.exist')
    cy.request({ url: `/sites/${siteID}`, failOnStatusCode: false }).its('status').should('eq', 404)
    cy.request({ method: 'POST', url: `/sites/${siteID}/delete`, form: true, body: {}, failOnStatusCode: false })
      .its('status').should('eq', 404)
    cy.visit('/')
    cy.location('pathname').should('eq', '/sites')
    cy.visit('/links')
    cy.contains('button', 'New link').should('be.disabled')
    cy.contains('Create your sites')
    cy.visit('/coverages')
    cy.contains('button', 'New coverage').should('be.disabled')
    cy.contains('Create one')
    cy.visit('/links/new')
    cy.contains('Create at least two')

    cy.login('alice', 'alice-password-1')
    cy.visit('/sites')
    cy.contains('tr', 'Alice site')
  })
})

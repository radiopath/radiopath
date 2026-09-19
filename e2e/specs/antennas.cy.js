describe('antennas', () => {
  before(() => {
    cy.wipe()
    cy.useradd('alice', 'alice-password-1')
  })
  beforeEach(() => cy.login('alice', 'alice-password-1'))

  it('imports an MSI file', () => {
    cy.visit('/antennas')
    cy.contains('No antennas yet')
    cy.get('input[type=file]').selectFile('fixtures/yagi.msi')
    cy.contains('button', 'Import').click()
    cy.contains('tr', 'E2E Yagi').within(() => {
      cy.contains('12.0 dBi')
      cy.contains('145 MHz')
      cy.get('svg')
    })
  })

  it('rejects a file without a pattern', () => {
    cy.visit('/antennas')
    cy.get('input[type=file]').selectFile({ contents: Cypress.Buffer.from('NAME x\nGAIN 3 dBd\n'), fileName: 'bad.msi' })
    cy.contains('button', 'Import').click()
    cy.contains('.alert-error li', 'no HORIZONTAL pattern')
    cy.contains('.table-note', '1 antenna')
  })

  it('builds a sector, edits and deletes it', () => {
    cy.visit('/antennas')
    cy.get('form[action="/antennas/manual"]').within(() => {
      cy.get('[name=name]').type('Sector 65')
      cy.get('[name=gain_dbi]').clear().type('15')
      cy.contains('button', 'Create').click()
    })
    cy.contains('tr', 'Sector 65').within(() => {
      cy.contains('15.0 dBi')
      cy.contains('25.0 dB')
    })
    cy.contains('tr', 'E2E Yagi').contains('Edit').should('not.exist')

    cy.contains('tr', 'Sector 65').contains('button', 'Edit').click()
    cy.get('dialog[open]').should('have.length', 1).within(() => {
      cy.get('[name=beamwidth_deg]').should('have.value', '65')
      cy.get('[name=front_back_db]').should('have.value', '25')
      cy.get('[name=name]').clear().type('Sector 90')
      cy.get('[name=gain_dbi]').clear().type('abc')
      cy.contains('button', 'Save').click()
    })
    cy.get('dialog[open]').within(() => {
      cy.contains('.alert-error li', 'Gain: not a number')
      cy.get('[name=name]').should('have.value', 'Sector 90')
      cy.get('[name=gain_dbi]').clear().type('17')
      cy.get('[name=beamwidth_deg]').clear().type('90')
      cy.get('[name=front_back_db]').clear().type('30')
      cy.contains('button', 'Save').click()
    })
    cy.get('dialog[open]').should('not.exist')
    cy.contains('Sector 65').should('not.exist')
    cy.contains('tr', 'Sector 90').within(() => {
      cy.contains('17.0 dBi')
      cy.contains('30.0 dB')
    })
    cy.contains('tr', 'Sector 90').contains('button', 'Edit').click()
    cy.get('dialog[open] [name=beamwidth_deg]').should('have.value', '90')
    cy.get('dialog[open]').contains('button', 'Close').click()
    cy.get('dialog[open]').should('not.exist')

    cy.contains('tr', 'Sector 90').contains('button', 'Delete').click()
    cy.contains('Sector 90').should('not.exist')
    cy.contains('.table-note', '1 antenna')
  })
})

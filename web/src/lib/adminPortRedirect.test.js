import { describe, expect, it } from 'vitest'
import { buildAdminPortUrl } from './adminPortRedirect'

describe('buildAdminPortUrl', () => {
  it('builds url with hash route', () => {
    expect(
      buildAdminPortUrl({
        host: '10.0.0.1',
        port: '9443',
        scheme: 'https:',
        pathname: '/',
        hash: '#/system/general',
      }),
    ).toBe('https://10.0.0.1:9443/#/system/general')
  })
})

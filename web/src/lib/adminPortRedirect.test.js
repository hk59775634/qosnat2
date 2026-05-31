import { describe, expect, it } from 'vitest'
import { buildAdminPortUrl, resolveTlsAccessHost } from './adminPortRedirect'

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

describe('resolveTlsAccessHost', () => {
  it('prefers access_host from API', () => {
    expect(resolveTlsAccessHost({ access_host: 'vpn.example.com' }, '1.2.3.4')).toBe('vpn.example.com')
  })

  it('falls back to cert_hostnames', () => {
    expect(resolveTlsAccessHost({ cert_hostnames: ['admin.example.com'] }, '1.2.3.4')).toBe('admin.example.com')
  })

  it('parses CN from cert_subject', () => {
    expect(resolveTlsAccessHost({ cert_subject: 'CN=vpn.test,O=Org' }, 'localhost')).toBe('vpn.test')
  })
})

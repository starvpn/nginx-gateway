import type { NgxDirective, NgxServer } from '@/api/ngx'
import { storeToRefs } from 'pinia'
import { useSiteEditorStore } from '../components/SiteEditor/store'

export function getListenPort(params = '') {
  const match = params.match(/(?:\[::\]:)?(\d+)/)
  return match?.[1] ?? ''
}

export function listenHasSSL(params = '') {
  return params.split(/\s+/).includes('ssl')
}

export function listenIsIPv6(params = '') {
  return params.trim().startsWith('[::]')
}

export function useServerDirectives() {
  const editorStore = useSiteEditorStore()
  const { ngxConfig, curServerIdx, curServer, curServerDirectives } = storeToRefs(editorStore)

  function ensureServer(): NgxServer {
    if (!ngxConfig.value.servers)
      ngxConfig.value.servers = []

    if (!curServer.value) {
      ngxConfig.value.servers.push({
        directives: [],
        locations: [],
      })
      curServerIdx.value = ngxConfig.value.servers.length - 1
    }

    if (!curServer.value.directives)
      curServer.value.directives = []

    if (!curServer.value.locations)
      curServer.value.locations = []

    return curServer.value
  }

  function ensureDirectives() {
    const server = ensureServer()
    if (!server.directives)
      server.directives = []

    return server.directives
  }

  function findDirective(directive: string, directives = curServerDirectives.value) {
    return directives?.find(item => item.directive === directive)
  }

  function findDirectives(directive: string, directives = curServerDirectives.value) {
    return directives?.filter(item => item.directive === directive) ?? []
  }

  function upsertDirective(directive: string, params: string, directives = ensureDirectives()) {
    const current = findDirective(directive, directives)
    if (current) {
      current.params = params
      return current
    }

    const item: NgxDirective = { directive, params }
    directives.push(item)
    return item
  }

  function removeDirective(directive: string, directives = ensureDirectives()) {
    for (let i = directives.length - 1; i >= 0; i--) {
      if (directives[i].directive === directive)
        directives.splice(i, 1)
    }
  }

  function removeDirectiveWhere(directive: string, predicate: (item: NgxDirective) => boolean, directives = ensureDirectives()) {
    for (let i = directives.length - 1; i >= 0; i--) {
      if (directives[i].directive === directive && predicate(directives[i]))
        directives.splice(i, 1)
    }
  }

  function getListenDirective(isSSL: boolean, isIPv6 = false, directives = curServerDirectives.value) {
    return findDirectives('listen', directives).find(item => {
      const params = item.params ?? ''
      return listenHasSSL(params) === isSSL && listenIsIPv6(params) === isIPv6
    })
  }

  function upsertListen(isSSL: boolean, params: string, directives = ensureDirectives()) {
    const current = getListenDirective(isSSL, listenIsIPv6(params), directives)
    if (current) {
      current.params = params
      return current
    }

    const item: NgxDirective = { directive: 'listen', params }
    directives.unshift(item)
    return item
  }

  return {
    ensureServer,
    ensureDirectives,
    findDirective,
    findDirectives,
    upsertDirective,
    removeDirective,
    removeDirectiveWhere,
    getListenDirective,
    getListenPort,
    upsertListen,
  }
}

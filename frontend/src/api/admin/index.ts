import dashboardAPI from './dashboard'
import groupsAPI from './groups'
import adminPlusAPI from './adminPlus'
import settingsAPI from './settings'
import systemAPI from './system'

export const adminAPI = {
  dashboard: dashboardAPI,
  groups: groupsAPI,
  adminPlus: adminPlusAPI,
  settings: settingsAPI,
  system: systemAPI
}

export {
  dashboardAPI,
  groupsAPI,
  adminPlusAPI,
  settingsAPI,
  systemAPI
}

export default adminAPI

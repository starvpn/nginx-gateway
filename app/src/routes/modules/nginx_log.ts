import type { RouteRecordRaw } from 'vue-router'
import { FileTextOutlined } from '@ant-design/icons-vue'

export const nginxLogRoutes: RouteRecordRaw[] = [
  {
    path: 'statistics_report',
    name: 'Statistics Report Redirect',
    redirect: '/dashboard/traffic_analysis',
    meta: {
      name: () => $gettext('流量分析'),
      hiddenInSidebar: true,
    },
  },
  {
    path: 'nginx_log',
    name: 'Nginx Log',
    meta: {
      name: () => $gettext('Nginx Log'),
      icon: FileTextOutlined,
    },
    children: [{
      path: 'access',
      name: 'Access Logs',
      component: () => import('@/views/nginx_log/NginxLog.vue'),
      meta: {
        name: () => $gettext('Access Logs'),
      },
    }, {
      path: 'error',
      name: 'Error Logs',
      component: () => import('@/views/nginx_log/NginxLog.vue'),
      meta: {
        name: () => $gettext('Error Logs'),
      },
    }, {
      path: 'site',
      name: 'Site Logs',
      component: () => import('@/views/nginx_log/NginxLog.vue'),
      meta: {
        name: () => $gettext('Site Logs'),
        hiddenInSidebar: true,
      },
    }, {
      path: 'list',
      name: 'Log List',
      component: () => import('@/views/nginx_log/NginxLogList.vue'),
      meta: {
        name: () => $gettext('Log List'),
      },
    }],
  },
]

// Minimal service worker — needed for showNotification() on mobile browsers
self.addEventListener('notificationclick', e => e.notification.close());

package com.gowebapp.app;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.content.Context;
import android.webkit.JavascriptInterface;

public class NotificationBridge {
    private static final String CHANNEL_ID = "app_channel";
    private final Context context;
    private int nextId = 0;

    public NotificationBridge(Context context) {
        this.context = context;
        NotificationManager nm = (NotificationManager)
            context.getSystemService(Context.NOTIFICATION_SERVICE);
        NotificationChannel channel = new NotificationChannel(
            CHANNEL_ID, "App Notifications", NotificationManager.IMPORTANCE_DEFAULT);
        nm.createNotificationChannel(channel);
    }

    @JavascriptInterface
    public void show(String title, String body) {
        NotificationManager nm = (NotificationManager)
            context.getSystemService(Context.NOTIFICATION_SERVICE);
        int icon = context.getResources().getIdentifier(
            "notification_icon", "drawable", context.getPackageName());
        if (icon == 0) icon = android.R.drawable.ic_popup_reminder;
        Notification notif = new Notification.Builder(context, CHANNEL_ID)
            .setSmallIcon(icon)
            .setContentTitle(title)
            .setContentText(body)
            .setAutoCancel(true)
            .build();
        nm.notify(nextId++, notif);
    }
}

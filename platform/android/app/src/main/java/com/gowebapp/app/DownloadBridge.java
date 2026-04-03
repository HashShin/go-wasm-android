package com.gowebapp.app;

import android.app.DownloadManager;
import android.content.Context;
import android.content.SharedPreferences;
import android.net.Uri;
import android.webkit.JavascriptInterface;

public class DownloadBridge {
    private static final String PREFS   = "download_prefs";
    private static final String KEY_DIR = "download_dir";

    private final Context context;
    private final String  defaultDir;

    public DownloadBridge(Context context, String defaultDir) {
        this.context    = context;
        this.defaultDir = defaultDir;
    }

    @JavascriptInterface
    public String getDownloadDir() {
        return context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
            .getString(KEY_DIR, defaultDir);
    }

    @JavascriptInterface
    public void setDownloadDir(String dir) {
        context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
            .edit().putString(KEY_DIR, dir).apply();
    }

    @JavascriptInterface
    public String download(String url, String filename) {
        try {
            String dir = getDownloadDir();
            DownloadManager.Request req = new DownloadManager.Request(Uri.parse(url));
            req.setDestinationInExternalPublicDir(dir, filename);
            req.setNotificationVisibility(
                DownloadManager.Request.VISIBILITY_VISIBLE_NOTIFY_COMPLETED);
            req.setTitle(filename);
            req.setDescription("Downloading…");
            DownloadManager dm = (DownloadManager)
                context.getSystemService(Context.DOWNLOAD_SERVICE);
            long id = dm.enqueue(req);
            return "Downloading → /" + dir + "/" + filename + "  (ID " + id + ")";
        } catch (Exception e) {
            return "error: " + e.getMessage();
        }
    }
}

package com.gowebapp.app;

import android.app.Activity;
import android.app.DownloadManager;
import android.content.Intent;
import android.content.SharedPreferences;
import android.net.Uri;
import android.provider.DocumentsContract;
import android.webkit.JavascriptInterface;
import android.webkit.WebView;
import java.io.InputStream;
import java.io.OutputStream;

public class DownloadBridge {
    static final int REQUEST_PICK_FOLDER = 42;

    private static final String PREFS   = "download_prefs";
    private static final String KEY_DIR = "download_dir";
    private static final String KEY_URI = "download_uri";

    private final Activity activity;
    private final WebView  webView;
    private final String   defaultDir;

    public DownloadBridge(Activity activity, WebView webView, String defaultDir) {
        this.activity   = activity;
        this.webView    = webView;
        this.defaultDir = defaultDir;
    }

    // ── Called from JS ────────────────────────────────────────────────────────

    @JavascriptInterface
    public void pickFolder() {
        Intent intent = new Intent(Intent.ACTION_OPEN_DOCUMENT_TREE);
        activity.startActivityForResult(intent, REQUEST_PICK_FOLDER);
    }

    @JavascriptInterface
    public String getDownloadDir() {
        return prefs().getString(KEY_DIR, defaultDir);
    }

    @JavascriptInterface
    public String download(String url, String filename) {
        String uriStr = prefs().getString(KEY_URI, null);
        if (uriStr != null) {
            return downloadToSaf(url, filename, Uri.parse(uriStr));
        }
        return downloadWithManager(url, filename, prefs().getString(KEY_DIR, defaultDir));
    }

    // ── Called from MainActivity.onActivityResult ─────────────────────────────

    public void onFolderPicked(Uri treeUri) {
        activity.getContentResolver().takePersistableUriPermission(treeUri,
            Intent.FLAG_GRANT_READ_URI_PERMISSION | Intent.FLAG_GRANT_WRITE_URI_PERMISSION);

        String displayPath = uriToDisplay(treeUri);
        prefs().edit()
            .putString(KEY_URI, treeUri.toString())
            .putString(KEY_DIR, displayPath)
            .apply();

        webView.post(() -> webView.evaluateJavascript(
            "if(window.onFolderPicked)onFolderPicked(" + quote(displayPath) + ")", null));
    }

    // ── Internal ──────────────────────────────────────────────────────────────

    private String downloadWithManager(String url, String filename, String dir) {
        try {
            DownloadManager.Request req = new DownloadManager.Request(Uri.parse(url));
            req.setDestinationInExternalPublicDir(dir, filename);
            req.setNotificationVisibility(
                DownloadManager.Request.VISIBILITY_VISIBLE_NOTIFY_COMPLETED);
            req.setTitle(filename);
            req.setDescription("Downloading\u2026");
            DownloadManager dm = (DownloadManager)
                activity.getSystemService(Activity.DOWNLOAD_SERVICE);
            long id = dm.enqueue(req);
            return "Downloading \u2192 /" + dir + "/" + filename + "  (ID " + id + ")";
        } catch (Exception e) {
            return "error: " + e.getMessage();
        }
    }

    private String downloadToSaf(String url, String filename, Uri treeUri) {
        new Thread(() -> {
            try {
                Uri docUri = DocumentsContract.buildDocumentUriUsingTree(
                    treeUri, DocumentsContract.getTreeDocumentId(treeUri));
                Uri fileUri = DocumentsContract.createDocument(
                    activity.getContentResolver(), docUri,
                    "application/octet-stream", filename);
                if (fileUri == null) { notifyResult("error: cannot create file"); return; }

                java.net.HttpURLConnection conn =
                    (java.net.HttpURLConnection) new java.net.URL(url).openConnection();
                InputStream  in  = conn.getInputStream();
                OutputStream out = activity.getContentResolver().openOutputStream(fileUri);
                byte[] buf = new byte[8192];
                int n, total = 0;
                while ((n = in.read(buf)) != -1) { out.write(buf, 0, n); total += n; }
                out.close(); in.close(); conn.disconnect();
                notifyResult("Saved: " + filename + " (" + (total / 1024) + " KB)");
            } catch (Exception e) {
                notifyResult("error: " + e.getMessage());
            }
        }).start();
        return "Downloading\u2026";
    }

    private void notifyResult(String msg) {
        webView.post(() -> webView.evaluateJavascript(
            "if(window.onDownloadResult)onDownloadResult(" + quote(msg) + ")", null));
    }

    private SharedPreferences prefs() {
        return activity.getSharedPreferences(PREFS, Activity.MODE_PRIVATE);
    }

    private static String uriToDisplay(Uri treeUri) {
        String docId = DocumentsContract.getTreeDocumentId(treeUri);
        if (docId.contains(":")) {
            String path = docId.split(":", 2)[1];
            return path.isEmpty() ? "Internal Storage" : path;
        }
        return docId;
    }

    private static String quote(String s) {
        return "\"" + s.replace("\\", "\\\\").replace("\"", "\\\"") + "\"";
    }
}

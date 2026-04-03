package com.gowebapp.app;

import android.app.Activity;
import android.content.Intent;
import android.content.pm.PackageManager;
import android.os.Build;
import android.os.Bundle;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;

public class MainActivity extends Activity {

    private WebView webView;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);

        webView = findViewById(R.id.webview);

        WebSettings settings = webView.getSettings();
        settings.setJavaScriptEnabled(true);
        settings.setAllowFileAccessFromFileURLs(true);
        settings.setAllowUniversalAccessFromFileURLs(true);
        settings.setDomStorageEnabled(true);

        webView.setWebViewClient(new WebViewClient());
        webView.addJavascriptInterface(new NotificationBridge(this), "AndroidNotification");

        // Request POST_NOTIFICATIONS permission on Android 13+
        if (Build.VERSION.SDK_INT >= 33 &&
                checkSelfPermission("android.permission.POST_NOTIFICATIONS")
                    != PackageManager.PERMISSION_GRANTED) {
            requestPermissions(new String[]{"android.permission.POST_NOTIFICATIONS"}, 1);
        }

        // Start loading immediately — runs in background while splash shows
        webView.loadUrl("file:///android_asset/index.html");

        // Launch splash on top if enabled (splash covers us while WASM loads)
        if (getResources().getBoolean(R.bool.splash_enabled)) {
            startActivity(new Intent(this, SplashActivity.class));
        }
    }

    @Override
    public void onBackPressed() {
        if (webView.canGoBack()) {
            webView.goBack();
        } else {
            super.onBackPressed();
        }
    }
}

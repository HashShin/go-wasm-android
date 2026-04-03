package com.gowebapp.app;

import android.app.Activity;
import android.content.Intent;
import android.content.pm.PackageManager;
import android.os.Build;
import android.os.Bundle;
import android.webkit.GeolocationPermissions;
import android.webkit.PermissionRequest;
import android.webkit.WebChromeClient;
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
        settings.setGeolocationEnabled(true);
        settings.setMediaPlaybackRequiresUserGesture(false);

        webView.setWebViewClient(new WebViewClient());
        webView.setWebChromeClient(new WebChromeClient() {
            @Override
            public void onPermissionRequest(PermissionRequest request) {
                // Grant camera/microphone access to the WebView
                request.grant(request.getResources());
            }

            @Override
            public void onGeolocationPermissionsShowPrompt(
                    String origin, GeolocationPermissions.Callback callback) {
                callback.invoke(origin, true, false);
            }
        });

        webView.addJavascriptInterface(new NotificationBridge(this), "AndroidNotification");
        webView.addJavascriptInterface(new StorageBridge(this), "AndroidStorage");
        webView.addJavascriptInterface(new MicrophoneBridge(), "AndroidMicrophone");

        // Request runtime permissions
        String[] perms = {
            "android.permission.CAMERA",
            "android.permission.RECORD_AUDIO",
            "android.permission.ACCESS_FINE_LOCATION",
            "android.permission.ACCESS_COARSE_LOCATION",
        };
        if (Build.VERSION.SDK_INT >= 33) {
            String[] with33 = new String[perms.length + 1];
            System.arraycopy(perms, 0, with33, 0, perms.length);
            with33[perms.length] = "android.permission.POST_NOTIFICATIONS";
            perms = with33;
        }
        boolean needsRequest = false;
        for (String p : perms) {
            if (checkSelfPermission(p) != PackageManager.PERMISSION_GRANTED) {
                needsRequest = true;
                break;
            }
        }
        if (needsRequest) requestPermissions(perms, 1);

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

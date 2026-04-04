package com.gowebapp.app;

import android.app.Activity;
import android.graphics.drawable.ColorDrawable;
import android.os.Bundle;
import android.os.Handler;
import android.view.animation.Animation;
import android.view.animation.AnimationUtils;
import android.webkit.JavascriptInterface;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.widget.ImageView;
import java.io.IOException;

public class SplashActivity extends Activity {

    private final Handler handler = new Handler();
    private Runnable dismissTask;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        // Apply splash background immediately so the window never flashes white.
        int bgColor = getResources().getColor(R.color.splash_background, getTheme());
        getWindow().setBackgroundDrawable(new ColorDrawable(bgColor));

        int duration = getResources().getInteger(R.integer.splash_duration);

        if (hasCustomSplash()) {
            showHtmlSplash(duration, bgColor);
        } else {
            showNativeSplash(duration);
        }
    }

    // ── HTML splash (app/splash.html) ─────────────────────────────────────────

    private void showHtmlSplash(int duration, int bgColor) {
        WebView webView = new WebView(this);
        webView.setBackgroundColor(bgColor);

        WebSettings s = webView.getSettings();
        s.setJavaScriptEnabled(true);
        s.setAllowFileAccessFromFileURLs(true);

        webView.addJavascriptInterface(new SplashBridge(), "SplashBridge");
        setContentView(webView);
        webView.loadUrl("file:///android_asset/splash.html");

        dismissTask = this::dismiss;
        handler.postDelayed(dismissTask, duration);
    }

    // ── Native splash (layout + optional animation) ───────────────────────────

    private void showNativeSplash(int duration) {
        setContentView(R.layout.activity_splash);

        if (getResources().getBoolean(R.bool.splash_animation)) {
            ImageView icon = findViewById(R.id.splash_icon);
            Animation anim = AnimationUtils.loadAnimation(this, R.anim.splash_in);
            icon.startAnimation(anim);
        }

        handler.postDelayed(this::dismiss, duration);
    }

    // ── Dismiss with fade-out transition ──────────────────────────────────────

    private void dismiss() {
        finish();
        overridePendingTransition(0, R.anim.splash_out);
    }

    // ── Helpers ───────────────────────────────────────────────────────────────

    private boolean hasCustomSplash() {
        try {
            getAssets().open("splash.html").close();
            return true;
        } catch (IOException e) {
            return false;
        }
    }

    class SplashBridge {
        @JavascriptInterface
        public void done() {
            handler.removeCallbacks(dismissTask);
            runOnUiThread(() -> dismiss());
        }
    }

    @Override
    protected void onDestroy() {
        handler.removeCallbacksAndMessages(null);
        super.onDestroy();
    }
}

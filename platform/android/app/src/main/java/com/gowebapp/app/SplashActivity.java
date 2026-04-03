package com.gowebapp.app;

import android.app.Activity;
import android.content.Intent;
import android.os.Bundle;
import android.os.Handler;
import android.view.animation.Animation;
import android.view.animation.AnimationUtils;
import android.widget.ImageView;

public class SplashActivity extends Activity {

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        if (!getResources().getBoolean(R.bool.splash_enabled)) {
            launch();
            return;
        }

        setContentView(R.layout.activity_splash);

        if (getResources().getBoolean(R.bool.splash_animation)) {
            ImageView icon = findViewById(R.id.splash_icon);
            Animation anim = AnimationUtils.loadAnimation(this, R.anim.splash_in);
            icon.startAnimation(anim);
        }

        int duration = getResources().getInteger(R.integer.splash_duration);
        new Handler().postDelayed(this::launch, duration);
    }

    private void launch() {
        startActivity(new Intent(this, MainActivity.class));
        finish();
    }
}

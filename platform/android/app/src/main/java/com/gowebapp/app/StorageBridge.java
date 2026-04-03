package com.gowebapp.app;

import android.content.Context;
import android.webkit.JavascriptInterface;
import java.io.BufferedReader;
import java.io.File;
import java.io.FileReader;
import java.io.FileWriter;

public class StorageBridge {
    private final Context context;

    public StorageBridge(Context context) {
        this.context = context;
    }

    @JavascriptInterface
    public String writeFile(String filename, String content) {
        try {
            File dir = context.getExternalFilesDir(null);
            if (dir == null) return "error: external storage unavailable";
            File file = new File(dir, filename);
            FileWriter writer = new FileWriter(file);
            writer.write(content);
            writer.close();
            return "written: " + file.getAbsolutePath();
        } catch (Exception e) {
            return "error: " + e.getMessage();
        }
    }

    @JavascriptInterface
    public String readFile(String filename) {
        try {
            File dir = context.getExternalFilesDir(null);
            if (dir == null) return "error: external storage unavailable";
            File file = new File(dir, filename);
            if (!file.exists()) return "error: file not found — write first";
            BufferedReader reader = new BufferedReader(new FileReader(file));
            StringBuilder sb = new StringBuilder();
            String line;
            while ((line = reader.readLine()) != null) sb.append(line).append("\n");
            reader.close();
            return sb.toString().trim();
        } catch (Exception e) {
            return "error: " + e.getMessage();
        }
    }
}

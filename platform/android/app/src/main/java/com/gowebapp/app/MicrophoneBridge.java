package com.gowebapp.app;

import android.media.AudioFormat;
import android.media.AudioRecord;
import android.media.MediaRecorder;
import android.webkit.JavascriptInterface;

public class MicrophoneBridge {
    private static final int SAMPLE_RATE = 44100;
    private static final int BUFFER_SIZE = AudioRecord.getMinBufferSize(
        SAMPLE_RATE, AudioFormat.CHANNEL_IN_MONO, AudioFormat.ENCODING_PCM_16BIT);

    private AudioRecord recorder = null;
    private volatile boolean recording = false;
    private volatile double amplitude = 0;

    @JavascriptInterface
    public void start() {
        if (recording) return;
        recorder = new AudioRecord(
            MediaRecorder.AudioSource.MIC,
            SAMPLE_RATE,
            AudioFormat.CHANNEL_IN_MONO,
            AudioFormat.ENCODING_PCM_16BIT,
            BUFFER_SIZE);
        recorder.startRecording();
        recording = true;
        new Thread(() -> {
            short[] buf = new short[BUFFER_SIZE];
            while (recording) {
                int read = recorder.read(buf, 0, buf.length);
                if (read > 0) {
                    long sum = 0;
                    for (int i = 0; i < read; i++) sum += (long) buf[i] * buf[i];
                    amplitude = Math.sqrt((double) sum / read) / 32768.0;
                }
            }
        }).start();
    }

    @JavascriptInterface
    public void stop() {
        recording = false;
        if (recorder != null) {
            recorder.stop();
            recorder.release();
            recorder = null;
        }
        amplitude = 0;
    }

    @JavascriptInterface
    public double getAmplitude() {
        return amplitude;
    }
}

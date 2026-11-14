package com.hayoonjae.asciiweb

import android.graphics.Bitmap
import android.graphics.BitmapFactory
import com.hayoonjae.asciiweb.engine.Thumb
import com.hayoonjae.asciiweb.engine.Thumbs
import kotlin.math.max
import kotlin.math.min

/** Decodes images with BitmapFactory, straight to a small RGB thumbnail (low memory). */
object ThumbDecoder {
    fun install() {
        Thumbs.decoder = ::decode
    }

    private fun decode(bytes: ByteArray): Thumb? {
        val bo = BitmapFactory.Options().apply { inJustDecodeBounds = true }
        BitmapFactory.decodeByteArray(bytes, 0, bytes.size, bo)
        val ow = bo.outWidth
        val oh = bo.outHeight
        if (ow <= 0 || oh <= 0 || ow.toLong() * oh > 16_000_000L) return null
        var sample = 1
        while (ow / (sample * 2) >= Thumbs.MAX_W * 2 && oh / (sample * 2) >= 2) sample *= 2
        val bmp = BitmapFactory.decodeByteArray(bytes, 0, bytes.size, BitmapFactory.Options().apply { inSampleSize = sample })
            ?: return null
        var tw = min(ow, Thumbs.MAX_W)
        var th = oh * tw / ow
        if (th > Thumbs.MAX_H) {
            th = Thumbs.MAX_H
            tw = ow * th / oh
        }
        tw = max(1, tw)
        th = max(1, th)
        val scaled = Bitmap.createScaledBitmap(bmp, tw, th, true)
        if (scaled !== bmp) bmp.recycle()
        val px = IntArray(tw * th)
        scaled.getPixels(px, 0, tw, 0, 0, tw, th)
        scaled.recycle()
        val pix = ByteArray(tw * th * 3)
        for (i in px.indices) {
            val c = px[i]
            val a = c ushr 24
            // composite on white paper
            pix[i * 3] = (((c shr 16 and 255) * a + 255 * (255 - a)) / 255).toByte()
            pix[i * 3 + 1] = (((c shr 8 and 255) * a + 255 * (255 - a)) / 255).toByte()
            pix[i * 3 + 2] = (((c and 255) * a + 255 * (255 - a)) / 255).toByte()
        }
        return Thumb(ow, oh, tw, th, pix)
    }
}

# Retrofit / Gson DTOs are accessed reflectively; keep their field names.
-keepattributes Signature
-keepattributes *Annotation*

# Keep Gson-serialized DTOs.
-keep class com.perfectgift.android.data.remote.dto.** { *; }

# Retrofit
-keepclasseswithmembers class * {
    @retrofit2.http.* <methods>;
}
-dontwarn okhttp3.**
-dontwarn retrofit2.**

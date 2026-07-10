plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.android)
    // To enable FCM push you must also add a google-services.json and apply:
    //   id("com.google.gms.google-services")
    // It is intentionally NOT applied here so the project builds without a
    // Firebase config file. See README "Push (FCM)".
}

android {
    namespace = "com.perfectgift.android"
    compileSdk = 34

    defaultConfig {
        applicationId = "com.perfectgift.android"
        minSdk = 26
        targetSdk = 34
        versionCode = 1
        versionName = "1.0"

        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
        vectorDrawables { useSupportLibrary = true }
    }

    // Gateway base URL is configured per build variant. The Android emulator reaches
    // the host machine's localhost via the special alias 10.0.2.2 (see README).
    buildTypes {
        debug {
            buildConfigField("String", "GATEWAY_BASE_URL", "\"http://10.0.2.2:8080/\"")
            isDebuggable = true
        }
        release {
            buildConfigField("String", "GATEWAY_BASE_URL", "\"https://api.perfectgift.app/\"")
            isMinifyEnabled = false
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro",
            )
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    kotlinOptions {
        jvmTarget = "17"
    }
    buildFeatures {
        compose = true
        buildConfig = true
    }
    composeOptions {
        kotlinCompilerExtensionVersion = libs.versions.composeCompiler.get()
    }
    packaging {
        resources {
            excludes += "/META-INF/{AL2.0,LGPL2.1}"
        }
    }
    testOptions {
        unitTests {
            isReturnDefaultValues = true
        }
    }
}

dependencies {
    implementation(libs.androidx.core.ktx)
    implementation(libs.androidx.lifecycle.runtime.ktx)
    implementation(libs.androidx.lifecycle.viewmodel.compose)
    implementation(libs.androidx.lifecycle.runtime.compose)
    implementation(libs.androidx.activity.compose)
    implementation(libs.androidx.navigation.compose)

    implementation(platform(libs.androidx.compose.bom))
    implementation(libs.androidx.compose.ui)
    implementation(libs.androidx.compose.ui.graphics)
    implementation(libs.androidx.compose.ui.tooling.preview)
    implementation(libs.androidx.compose.material3)
    debugImplementation(libs.androidx.compose.ui.tooling)

    implementation(libs.androidx.datastore.preferences)
    implementation(libs.kotlinx.coroutines.android)

    implementation(libs.retrofit)
    implementation(libs.retrofit.converter.gson)
    implementation(libs.okhttp)
    implementation(libs.okhttp.logging)
    implementation(libs.gson)

    // Push (FCM). Compiles without google-services.json; runtime delivery needs it.
    implementation(platform(libs.firebase.bom))
    implementation(libs.firebase.messaging)

    // Google Sign-In via Credential Manager (obtain a Google ID token → /v1/auth/signin).
    implementation(libs.androidx.credentials)
    implementation(libs.androidx.credentials.play.services)
    implementation(libs.googleid)

    testImplementation(libs.junit)
    testImplementation(libs.kotlinx.coroutines.test)
    testImplementation(libs.okhttp.mockwebserver)
    testImplementation(libs.retrofit.converter.gson)
}

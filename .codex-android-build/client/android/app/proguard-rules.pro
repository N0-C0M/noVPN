# Aggressive release hardening.
-repackageclasses "x"
-allowaccessmodification
-adaptclassstrings
-adaptresourcefilenames
-adaptresourcefilecontents

# JNI bridge uses symbol-based lookup (Java_com_novpn_vpn_Tun2ProxyBridge_*).
# Keep class and native member names stable.
-keep class com.novpn.vpn.Tun2ProxyBridge
-keepclassmembers class com.novpn.vpn.Tun2ProxyBridge {
    native <methods>;
}

# Keep attributes frequently required by Android/Kotlin tooling.
-keepattributes *Annotation*,InnerClasses,EnclosingMethod,Signature

# Strip verbose logs from release bytecode.
-assumenosideeffects class android.util.Log {
    public static int v(...);
    public static int d(...);
    public static int i(...);
    public static int w(...);
    public static int e(...);
    public static int println(...);
}

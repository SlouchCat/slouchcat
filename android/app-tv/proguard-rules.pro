# R8 rules of the TV app beyond those the libraries ship (kotlinx.serialization, OkHttp, Compose,
# Navigation, Tink bring their own). The generated client and the typed routes are @Serializable;
# their serializers are kept by the rules of kotlinx.serialization. Keep the names of the API models
# for readable errors in the field.
-keepnames class com.slouchcat.core.api.generated.models.** { *; }
# ZXing core: only the encoder is used, nothing reflective.

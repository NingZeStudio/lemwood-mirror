<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { ShieldCheck, Loader2, CheckCircle, XCircle, Copy, Download, Home } from 'lucide-vue-next';
import { getCaptchaConfig, verifyDownload } from '@/services/api';

const route = useRoute();
const router = useRouter();

const filePath = ref('');
const captchaId = ref('');
const isLoading = ref(true);
const isVerifying = ref(false);
const verifyStatus = ref('pending');
const errorMessage = ref('');
const downloadUrl = ref('');
const showCopiedTip = ref(false);
const downloadStarted = ref(false);

const fullDownloadUrl = computed(() => {
  if (downloadUrl.value) {
    return 'https://mirror.lemwood.icu' + downloadUrl.value;
  }
  return '';
});

let captchaObj = null;

const showCaptcha = () => {
  if (captchaObj) {
    isLoading.value = false;
    verifyStatus.value = 'pending';
    captchaObj.showCaptcha();
  }
};

const verifyCaptcha = async (lotNumber, captchaOutput, passToken, genTime) => {
  isVerifying.value = true;
  verifyStatus.value = 'pending';

  try {
    const response = await verifyDownload(lotNumber, captchaOutput, passToken, genTime, filePath.value);
    downloadUrl.value = response.data.download_url;
    verifyStatus.value = 'success';
    isLoading.value = false;
  } catch (error) {
    console.error('Verify download error:', error);
    errorMessage.value = error.response?.data?.message || '验证失败，请重试';
    verifyStatus.value = 'error';
    isLoading.value = false;
  } finally {
    isVerifying.value = false;
  }
};

const startDownload = () => {
  if (downloadUrl.value) {
    window.location.href = downloadUrl.value;
    downloadStarted.value = true;
  }
};

const goToHome = () => {
  router.push('/');
};

const copyUrl = async () => {
  if (downloadUrl.value) {
    const fullUrl = 'https://mirror.lemwood.icu' + downloadUrl.value;
    try {
      await navigator.clipboard.writeText(fullUrl);
      showCopiedTip.value = true;
      setTimeout(() => {
        showCopiedTip.value = false;
      }, 2000);
    } catch (err) {
      console.error('Copy failed:', err);
    }
  }
};

const loadCaptchaScript = () => {
  return new Promise((resolve, reject) => {
    if (window.initGeetest4) {
      resolve();
      return;
    }

    const existingScript = document.querySelector('script[src*="gt4.js"]');
    if (existingScript) {
      const checkLoaded = setInterval(() => {
        if (window.initGeetest4) {
          clearInterval(checkLoaded);
          resolve();
        }
      }, 100);
      return;
    }

    const script = document.createElement('script');
    script.src = 'https://static.geetest.com/v4/gt4.js';
    script.async = true;

    script.onload = () => resolve();
    script.onerror = () => reject(new Error('Failed to load Geetest script'));
    document.head.appendChild(script);
  });
};

const initCaptcha = () => {
  isLoading.value = true;
  verifyStatus.value = 'pending';

  window.initGeetest4({
    captchaId: captchaId.value,
    product: 'bind'
  }, (captcha) => {
    captchaObj = captcha;

    captcha.onReady(() => {
      isLoading.value = false;
      captcha.showCaptcha();
    });

    captcha.onSuccess(() => {
      const result = captcha.getValidate();
      console.log('Geetest validate result:', result);
      if (result && result.lot_number) {
        verifyCaptcha(result.lot_number, result.captcha_output, result.pass_token, result.gen_time);
      } else {
        isLoading.value = false;
        verifyStatus.value = 'error';
        errorMessage.value = '验证结果获取失败，请重试';
      }
    });

    captcha.onError((e) => {
      isLoading.value = false;
      verifyStatus.value = 'error';
      errorMessage.value = '验证加载失败: ' + (e.msg || '未知错误');
    });

    captcha.onClose(() => {
      isLoading.value = false;
      verifyStatus.value = 'error';
      errorMessage.value = '用户取消验证';
    });
  });
};

const init = async () => {
  isLoading.value = true;
  filePath.value = route.query.file || '';

  if (!filePath.value) {
    errorMessage.value = '缺少文件参数';
    verifyStatus.value = 'error';
    isLoading.value = false;
    return;
  }

  try {
    const [response] = await Promise.all([
      getCaptchaConfig(),
      loadCaptchaScript()
    ]);
    
    captchaId.value = response.data.app_id;
    console.log('Config loaded:', { enabled: response.data.enabled, captchaId: captchaId.value });

    if (!response.data.enabled) {
      window.location.href = `/download/${filePath.value}`;
      return;
    }

    initCaptcha();
  } catch (error) {
    console.error('Init error:', error);
    errorMessage.value = '加载配置失败: ' + error.message;
    verifyStatus.value = 'error';
    isLoading.value = false;
  }
};

onMounted(() => {
  init();
});

onUnmounted(() => {
  if (captchaObj) {
    captchaObj.destroy();
  }
});
</script>

<template>
  <div class="verify-page">
    <div v-if="showCopiedTip" class="copied-tip">已复制到剪贴板</div>
    <div class="verify-container">
      <div class="verify-card">
        <div class="verify-header">
          <ShieldCheck class="h-12 w-12 text-primary" />
          <h1>安全验证</h1>
          <p>请完成验证后开始下载</p>
        </div>

        <div class="verify-content">
          <div v-if="isLoading && verifyStatus !== 'error'" class="verify-loading">
            <Loader2 class="h-8 w-8 animate-spin" />
            <span>正在加载验证...</span>
          </div>

          <div v-else-if="verifyStatus === 'success'" class="verify-success">
            <CheckCircle class="h-16 w-16 text-green-500" />
            <span>验证成功</span>
            <div class="download-url-box">
              {{ fullDownloadUrl }}
            </div>
            <div class="btn-group">
              <button v-if="!downloadStarted" @click="startDownload" class="btn-primary">
                <Download class="h-4 w-4 mr-2" />
                直接下载
              </button>
              <button v-else @click="goToHome" class="btn-primary">
                <Home class="h-4 w-4 mr-2" />
                返回首页
              </button>
              <button @click="copyUrl" class="btn-secondary">
                <Copy class="h-4 w-4 mr-2" />
                复制链接
              </button>
            </div>
          </div>

          <div v-else-if="verifyStatus === 'error'" class="verify-error">
            <XCircle class="h-16 w-16 text-red-500" />
            <span>{{ errorMessage }}</span>
            <button @click="showCaptcha" class="retry-btn">重新验证</button>
          </div>
        </div>

        <div class="verify-footer">
          <p class="file-path" v-if="filePath">
            文件: {{ filePath.split('/').pop() }}
          </p>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.verify-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #f5f7fa 0%, #e4e8ec 100%);
  padding: 20px;
  position: relative;
}

@media (prefers-color-scheme: dark) {
  .verify-page {
    background: linear-gradient(135deg, #1a1a2e 0%, #16213e 100%);
  }
}

.copied-tip {
  position: fixed;
  top: 20px;
  left: 50%;
  transform: translateX(-50%);
  background: #22c55e;
  color: white;
  padding: 8px 16px;
  border-radius: 8px;
  font-size: 14px;
  z-index: 1000;
  animation: fadeInOut 2s ease-in-out;
}

@keyframes fadeInOut {
  0%, 100% { opacity: 0; }
  10%, 90% { opacity: 1; }
}

.verify-container {
  width: 100%;
  max-width: 480px;
}

.verify-card {
  background: #ffffff;
  border-radius: 16px;
  box-shadow: 0 20px 40px rgba(0, 0, 0, 0.1);
  overflow: hidden;
}

@media (prefers-color-scheme: dark) {
  .verify-card {
    background: #1f2937;
  }
}

.verify-header {
  text-align: center;
  padding: 32px 24px 24px;
  border-bottom: 1px solid #e5e7eb;
}

@media (prefers-color-scheme: dark) {
  .verify-header {
    border-bottom-color: #374151;
  }
}

.verify-header h1 {
  margin: 16px 0 8px;
  font-size: 24px;
  font-weight: 600;
  color: #111827;
}

@media (prefers-color-scheme: dark) {
  .verify-header h1 {
    color: #f3f4f6;
  }
}

.verify-header p {
  color: #6b7280;
  font-size: 14px;
}

.verify-content {
  padding: 32px 24px;
  min-height: 200px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.verify-loading,
.verify-success,
.verify-error {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
  text-align: center;
  width: 100%;
}

.verify-success span,
.verify-error span {
  color: #6b7280;
  font-size: 14px;
}

.download-url-box {
  width: 100%;
  margin-top: 8px;
  padding: 12px;
  background: #f3f4f6;
  border-radius: 8px;
  font-size: 12px;
  word-break: break-all;
  color: #374151;
  text-align: left;
}

@media (prefers-color-scheme: dark) {
  .download-url-box {
    background: #374151;
    color: #e5e7eb;
  }
}

.btn-group {
  display: flex;
  gap: 8px;
  margin-top: 16px;
  width: 100%;
}

.btn-group button {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 10px 16px;
  border: none;
  border-radius: 8px;
  cursor: pointer;
  font-size: 14px;
  transition: opacity 0.2s;
}

.btn-group button:hover {
  opacity: 0.9;
}

.btn-primary {
  background: #3b82f6;
  color: white;
}

.btn-secondary {
  background: #e5e7eb;
  color: #374151;
}

@media (prefers-color-scheme: dark) {
  .btn-secondary {
    background: #4b5563;
    color: #f3f4f6;
  }
}

.retry-btn {
  margin-top: 8px;
  padding: 8px 24px;
  background: #3b82f6;
  color: white;
  border: none;
  border-radius: 8px;
  cursor: pointer;
  font-size: 14px;
}

.retry-btn:hover {
  opacity: 0.9;
}

.verify-footer {
  padding: 16px 24px;
  border-top: 1px solid #e5e7eb;
  text-align: center;
}

@media (prefers-color-scheme: dark) {
  .verify-footer {
    border-top-color: #374151;
  }
}

.file-path {
  font-size: 12px;
  color: #6b7280;
  word-break: break-all;
}
</style>

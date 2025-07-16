package dto

type UpdateUserInput struct {
    Username *string `json:"username"`
    Password *string `json:"password"`
    Email    *string `json:"email"`
    FullName *string `json:"full_name"`
    Type     *string `json:"type"`
    Status   *string `json:"status"`
}
